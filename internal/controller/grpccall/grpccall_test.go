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
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	grpcv1alpha1 "github.com/crossplane/provider-template/apis/grpc/v1alpha1"
	grpcclient "github.com/crossplane/provider-template/internal/grpc"
)

// Unlike many Kubernetes projects Crossplane does not use third party testing
// libraries, per the common Go test review comments:
// https://github.com/golang/go/wiki/CodeReviewComments#useful-test-failures

func TestObserve(t *testing.T) {
	type fields struct {
		client MockGRPCClient
	}

	type args struct {
		ctx context.Context
		mg  resource.Managed
	}

	type want struct {
		o   managed.ExternalObservation
		err error
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{
		"ValidMethod": {
			reason: "Should return ResourceExists=true for valid methods",
			fields: fields{
				client: MockGRPCClient{
					MockValidateMethod: func(serviceName, methodName string) error {
						return nil
					},
				},
			},
			args: args{
				ctx: context.Background(),
				mg: &grpcv1alpha1.GrpcCall{
					Spec: grpcv1alpha1.GrpcCallSpec{
						ForProvider: grpcv1alpha1.GrpcCallParameters{
							ServiceName: "test.Service",
							MethodName:  "TestMethod",
						},
					},
				},
			},
			want: want{
				o: managed.ExternalObservation{
					ResourceExists:    true,
					ResourceUpToDate:  true,
					ConnectionDetails: managed.ConnectionDetails{},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := &external{client: &tc.fields.client}
			o, err := e.Observe(tc.args.ctx, tc.args.mg)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Observe(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.o, o); diff != "" {
				t.Errorf("\n%s\ne.Observe(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

// CallResult represents the result of a GRPC call for testing
type CallResult = grpcclient.CallResult

// MockGRPCClient is a mock implementation of the GRPC client for testing
type MockGRPCClient struct {
	MockValidateMethod func(serviceName, methodName string) error
	MockCallMethod     func(ctx context.Context, serviceName, methodName string, requestData runtime.RawExtension, headers map[string]string, timeout *int32) (*CallResult, error)
	MockClose          func() error
}

func (m *MockGRPCClient) ValidateMethod(serviceName, methodName string) error {
	return m.MockValidateMethod(serviceName, methodName)
}

func (m *MockGRPCClient) CallMethod(ctx context.Context, serviceName, methodName string, requestData runtime.RawExtension, headers map[string]string, timeout *int32) (*CallResult, error) {
	return m.MockCallMethod(ctx, serviceName, methodName, requestData, headers, timeout)
}

func (m *MockGRPCClient) Close() error {
	return m.MockClose()
}
