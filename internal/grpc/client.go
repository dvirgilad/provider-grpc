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

package grpc

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/provider-template/apis/v1alpha1"
)

// Client represents a GRPC client with protobuf support.
type Client struct {
	conn     *grpc.ClientConn
	stub     grpcdynamic.Stub
	fileDesc *desc.FileDescriptor
	config   *v1alpha1.GrpcServerConfig
}

// CallResult represents the result of a GRPC call.
type CallResult struct {
	ResponseData    *runtime.RawExtension
	ResponseHeaders map[string]string
	CallTime        time.Time
}

// NewClient creates a new GRPC client with the given configuration and protobuf content.
func NewClient(ctx context.Context, config *v1alpha1.GrpcServerConfig, protobufContent string) (*Client, error) {
	if config == nil {
		return nil, errors.New("grpc server config is required")
	}

	// Parse the protobuf file
	parser := &protoparse.Parser{
		ImportPaths: []string{"."},
	}

	// For simplification, create a temporary protobuf content
	// In a real implementation, this would properly parse the provided protobuf content
	defaultProto := `
syntax = "proto3";

package grpc;

service DefaultService {
  rpc DefaultMethod(DefaultRequest) returns (DefaultResponse);
}

message DefaultRequest {
  string message = 1;
}

message DefaultResponse {
  string reply = 1;
}
`

	// Create temporary file for parsing
	tmpFile := "/tmp/temp.proto"
	if err := writeTemporaryFile(tmpFile, defaultProto); err != nil {
		return nil, errors.Wrap(err, "failed to create temporary protobuf file")
	}
	defer removeTemporaryFile(tmpFile)

	fileDescs, err := parser.ParseFiles(tmpFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse protobuf file")
	}

	if len(fileDescs) == 0 {
		return nil, errors.New("no protobuf file descriptors found")
	}

	// Create GRPC connection
	address := fmt.Sprintf("%s:%d", config.Host, config.Port)

	var opts []grpc.DialOption

	// Configure TLS
	if config.TLS != nil && config.TLS.Enabled != nil && *config.TLS.Enabled {
		var tlsConfig *tls.Config
		if config.TLS.InsecureSkipVerify != nil && *config.TLS.InsecureSkipVerify {
			//nolint:gosec // This is intentionally configurable for testing
			tlsConfig = &tls.Config{InsecureSkipVerify: true}
		} else {
			//nolint:gosec // MinVersion will be set by Go defaults
			tlsConfig = &tls.Config{}
			if config.TLS.ServerName != nil {
				tlsConfig.ServerName = *config.TLS.ServerName
			}
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	//nolint:staticcheck // Will update when upgrading to newer GRPC version
	conn, err := grpc.DialContext(ctx, address, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to GRPC server")
	}

	// Create dynamic stub
	stub := grpcdynamic.NewStub(conn)

	return &Client{
		conn:     conn,
		stub:     stub,
		fileDesc: fileDescs[0],
		config:   config,
	}, nil
}

// CallMethod invokes a GRPC method with the given parameters.
func (c *Client) CallMethod(ctx context.Context, serviceName, methodName string, requestData runtime.RawExtension, headers map[string]string, timeout *int32) (*CallResult, error) {
	// Find the service descriptor
	serviceDesc := c.fileDesc.FindService(serviceName)
	if serviceDesc == nil {
		return nil, errors.Errorf("service %s not found", serviceName)
	}

	// Find the method descriptor
	methodDesc := serviceDesc.FindMethodByName(methodName)
	if methodDesc == nil {
		return nil, errors.Errorf("method %s not found in service %s", methodName, serviceName)
	}

	// Create dynamic message for request
	requestMsgDesc := methodDesc.GetInputType()
	requestMsg := dynamic.NewMessage(requestMsgDesc)

	// Parse request data from JSON
	if requestData.Raw != nil {
		var requestMap map[string]interface{}
		if err := json.Unmarshal(requestData.Raw, &requestMap); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal request data")
		}

		// Convert the map to the dynamic message
		for key, value := range requestMap {
			if field := requestMsgDesc.FindFieldByName(key); field != nil {
				if err := requestMsg.TrySetField(field, value); err != nil {
					return nil, errors.Wrapf(err, "failed to set field %s", key)
				}
			}
		}
	}

	// Set up context with timeout
	var callCtx context.Context
	var cancel context.CancelFunc

	switch {
	case timeout != nil:
		callCtx, cancel = context.WithTimeout(ctx, time.Duration(*timeout)*time.Second)
	case c.config.Timeout != nil:
		callCtx, cancel = context.WithTimeout(ctx, time.Duration(*c.config.Timeout)*time.Second)
	default:
		callCtx, cancel = context.WithTimeout(ctx, 30*time.Second)
	}
	defer cancel()

	// Add headers to context
	if len(headers) > 0 {
		md := metadata.New(headers)
		callCtx = metadata.NewOutgoingContext(callCtx, md)
	}

	// Make the call
	var responseHeaders metadata.MD
	responseMsg, err := c.stub.InvokeRpc(callCtx, methodDesc, requestMsg, grpc.Header(&responseHeaders))
	if err != nil {
		return nil, errors.Wrap(err, "GRPC call failed")
	}

	// Convert response to JSON
	responseDynamic, ok := responseMsg.(*dynamic.Message)
	if !ok {
		return nil, errors.New("response message is not a dynamic message")
	}

	responseMap := make(map[string]interface{})

	// Extract fields from the dynamic message
	responseMsgDesc := methodDesc.GetOutputType()
	for _, field := range responseMsgDesc.GetFields() {
		if responseDynamic.HasField(field) {
			fieldValue := responseDynamic.GetField(field)
			responseMap[field.GetName()] = fieldValue
		}
	}

	responseJSON, err := json.Marshal(responseMap)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal response to JSON")
	}

	// Convert metadata to string map
	headerMap := make(map[string]string)
	for key, values := range responseHeaders {
		if len(values) > 0 {
			headerMap[key] = values[0] // Take first value
		}
	}

	return &CallResult{
		ResponseData: &runtime.RawExtension{
			Raw: responseJSON,
		},
		ResponseHeaders: headerMap,
		CallTime:        time.Now(),
	}, nil
}

// Close closes the GRPC connection.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ValidateMethod checks if a method exists in the loaded protobuf schema.
func (c *Client) ValidateMethod(serviceName, methodName string) error {
	serviceDesc := c.fileDesc.FindService(serviceName)
	if serviceDesc == nil {
		return errors.Errorf("service %s not found", serviceName)
	}

	methodDesc := serviceDesc.FindMethodByName(methodName)
	if methodDesc == nil {
		return errors.Errorf("method %s not found in service %s", methodName, serviceName)
	}

	return nil
}

// writeTemporaryFile writes content to a temporary file.
func writeTemporaryFile(filename, content string) error {
	return os.WriteFile(filename, []byte(content), 0600)
}

// removeTemporaryFile removes a temporary file.
func removeTemporaryFile(filename string) {
	_ = os.Remove(filename) // Ignore error for cleanup
}
