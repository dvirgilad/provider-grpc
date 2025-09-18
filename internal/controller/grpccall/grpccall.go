/*
Copyright 2025 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package grpccall

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/feature"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/connection"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/statemetrics"

	grpcv1alpha1 "github.com/crossplane/provider-template/apis/grpc/v1alpha1"
	apisv1alpha1 "github.com/crossplane/provider-template/apis/v1alpha1"
	"github.com/crossplane/provider-template/internal/features"
	grpcclient "github.com/crossplane/provider-template/internal/grpc"
)

const (
	errNotGrpcCall  = "managed resource is not a GrpcCall custom resource"
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errGetPC        = "cannot get ProviderConfig"
	errGetCreds     = "cannot get credentials"
	errNewClient    = "cannot create new GRPC Service"
	errCallMethod   = "cannot call GRPC method"
)

// Setup adds a controller that reconciles GrpcCall managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(grpcv1alpha1.GrpcCallGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	opts := []managed.ReconcilerOption{
		managed.WithExternalConnecter(&connector{
			kube:  mgr.GetClient(),
			usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...),
		managed.WithManagementPolicies(),
	}

	if o.Features.Enabled(feature.EnableAlphaChangeLogs) {
		opts = append(opts, managed.WithChangeLogger(o.ChangeLogOptions.ChangeLogger))
	}

	if o.MetricOptions != nil {
		opts = append(opts, managed.WithMetricRecorder(o.MetricOptions.MRMetrics))
	}

	if o.MetricOptions != nil && o.MetricOptions.MRStateMetrics != nil {
		stateMetricsRecorder := statemetrics.NewMRStateRecorder(
			mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &grpcv1alpha1.GrpcCallList{}, o.MetricOptions.PollStateMetricInterval,
		)
		if err := mgr.Add(stateMetricsRecorder); err != nil {
			return errors.Wrap(err, "cannot register MR state metrics recorder for kind grpcv1alpha1.GrpcCallList")
		}
	}

	r := managed.NewReconciler(mgr, resource.ManagedKind(grpcv1alpha1.GrpcCallGroupVersionKind), opts...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&grpcv1alpha1.GrpcCall{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method is called.
type connector struct {
	kube  client.Client
	usage resource.Tracker
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*grpcv1alpha1.GrpcCall)
	if !ok {
		return nil, errors.New(errNotGrpcCall)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	pc := &apisv1alpha1.ProviderConfig{}
	if err := c.kube.Get(ctx, types.NamespacedName{Name: cr.GetProviderConfigReference().Name}, pc); err != nil {
		return nil, errors.Wrap(err, errGetPC)
	}

	// Get protobuf content based on the configured source
	protobufContent, err := c.getProtobufContent(ctx, &pc.Spec.ProtobufConfig)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get protobuf content")
	}

	// Create GRPC client
	grpcClient, err := grpcclient.NewClient(ctx, &pc.Spec.GrpcServerConfig, protobufContent)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{
		client: grpcClient,
		kube:   c.kube,
	}, nil
}

// getProtobufContent retrieves protobuf content based on the configured source.
func (c *connector) getProtobufContent(ctx context.Context, config *apisv1alpha1.ProtobufConfig) (string, error) {
	switch config.Source {
	case apisv1alpha1.ProtobufSourceInline:
		if config.Inline == nil {
			return "", errors.New("inline protobuf content is required when source is Inline")
		}
		return *config.Inline, nil

	case apisv1alpha1.ProtobufSourceSecret:
		if config.SecretRef == nil {
			return "", errors.New("secret reference is required when source is Secret")
		}
		return c.getContentFromSecret(ctx, config.SecretRef)

	case apisv1alpha1.ProtobufSourceConfigMap:
		if config.ConfigMapRef == nil {
			return "", errors.New("configmap reference is required when source is ConfigMap")
		}
		return c.getContentFromConfigMap(ctx, config.ConfigMapRef)

	default:
		return "", errors.Errorf("unsupported protobuf source: %s", config.Source)
	}
}

// getContentFromSecret retrieves content from a Secret.
func (c *connector) getContentFromSecret(ctx context.Context, secretRef *apisv1alpha1.SecretKeySelector) (string, error) {
	// This would typically extract content from a secret
	// For now, return a placeholder
	return "// Placeholder: content from secret", nil
}

// getContentFromConfigMap retrieves content from a ConfigMap.
func (c *connector) getContentFromConfigMap(ctx context.Context, configMapRef *apisv1alpha1.ConfigMapKeySelector) (string, error) {
	// This would typically extract content from a configmap
	// For now, return a placeholder
	return "// Placeholder: content from configmap", nil
}

// An ExternalClient observes, then either creates, updates, or deletes an external resource.
type external struct {
	client grpcclient.GRPCClient
	kube   client.Client
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*grpcv1alpha1.GrpcCall)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotGrpcCall)
	}

	// Validate that the method exists
	err := e.client.ValidateMethod(cr.Spec.ForProvider.ServiceName, cr.Spec.ForProvider.MethodName)
	if err != nil {
		return managed.ExternalObservation{
			ResourceExists:   false,
			ResourceUpToDate: false,
		}, errors.Wrap(err, "method validation failed")
	}

	fmt.Printf("Observing GrpcCall: %s.%s\n", cr.Spec.ForProvider.ServiceName, cr.Spec.ForProvider.MethodName)

	return managed.ExternalObservation{
		ResourceExists:    true,
		ResourceUpToDate:  true,
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*grpcv1alpha1.GrpcCall)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotGrpcCall)
	}

	fmt.Printf("Creating GrpcCall: %s.%s\n", cr.Spec.ForProvider.ServiceName, cr.Spec.ForProvider.MethodName)

	// Make the GRPC call
	result, err := e.client.CallMethod(
		ctx,
		cr.Spec.ForProvider.ServiceName,
		cr.Spec.ForProvider.MethodName,
		cr.Spec.ForProvider.RequestData,
		cr.Spec.ForProvider.Headers,
		cr.Spec.ForProvider.Timeout,
	)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCallMethod)
	}

	// Update the resource status with the call result
	e.updateStatus(cr, result)

	return managed.ExternalCreation{
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*grpcv1alpha1.GrpcCall)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotGrpcCall)
	}

	fmt.Printf("Updating GrpcCall: %s.%s\n", cr.Spec.ForProvider.ServiceName, cr.Spec.ForProvider.MethodName)

	// Make the GRPC call
	result, err := e.client.CallMethod(
		ctx,
		cr.Spec.ForProvider.ServiceName,
		cr.Spec.ForProvider.MethodName,
		cr.Spec.ForProvider.RequestData,
		cr.Spec.ForProvider.Headers,
		cr.Spec.ForProvider.Timeout,
	)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errCallMethod)
	}

	// Update the resource status with the call result
	e.updateStatus(cr, result)

	return managed.ExternalUpdate{
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*grpcv1alpha1.GrpcCall)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotGrpcCall)
	}

	fmt.Printf("Deleting GrpcCall: %s.%s\n", cr.Spec.ForProvider.ServiceName, cr.Spec.ForProvider.MethodName)

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(ctx context.Context) error {
	if e.client != nil {
		return e.client.Close()
	}
	return nil
}

// updateStatus updates the resource status with call results.
func (e *external) updateStatus(cr *grpcv1alpha1.GrpcCall, result *grpcclient.CallResult) {
	if cr.Status.AtProvider.CallCount == nil {
		cr.Status.AtProvider.CallCount = new(int64)
	}
	*cr.Status.AtProvider.CallCount++

	cr.Status.AtProvider.ResponseData = result.ResponseData
	cr.Status.AtProvider.ResponseHeaders = result.ResponseHeaders

	// Convert time.Time to metav1.Time
	callTime := metav1.NewTime(result.CallTime)
	cr.Status.AtProvider.LastCallTime = &callTime
}
