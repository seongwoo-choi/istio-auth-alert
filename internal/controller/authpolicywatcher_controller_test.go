/*
Copyright 2025.

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

package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	apiSecurity "istio.io/api/security/v1beta1"
	clientSecurity "istio.io/client-go/pkg/apis/security/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestCollectFunctions validates that collectIPs and collectHosts work correctly.
func TestCollectFunctions(t *testing.T) {
	policy := &clientSecurity.AuthorizationPolicy{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "p"},
		Spec: apiSecurity.AuthorizationPolicy{
			Rules: []*apiSecurity.Rule{
				{
					From: []*apiSecurity.Rule_From{{Source: &apiSecurity.Source{RemoteIpBlocks: []string{"10.0.0.1"}}}},
					To:   []*apiSecurity.Rule_To{{Operation: &apiSecurity.Operation{Hosts: []string{"foo.com"}}}},
				},
			},
		},
	}
	// IPs
	gotIPs := collectIPs(policy)
	wantIPs := []string{"10.0.0.1"}
	if !reflect.DeepEqual(gotIPs, wantIPs) {
		t.Errorf("collectIPs = %v; want %v", gotIPs, wantIPs)
	}
	// Hosts
	gotHosts := collectHosts(policy)
	wantHosts := []string{"foo.com"}
	if !reflect.DeepEqual(gotHosts, wantHosts) {
		t.Errorf("collectHosts = %v; want %v", gotHosts, wantHosts)
	}
}

// TestReconcile_SlackAlert verifies that Reconcile sends a Slack alert.
func TestReconcile_SlackAlert(t *testing.T) {
	called := false
	var payload map[string]string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %s; want application/json", ct)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	os.Setenv("SLACK_WEBHOOK_URL", ts.URL)

	// Scheme setup
	scheme := runtime.NewScheme()
	clientgoscheme.AddToScheme(scheme)
	clientSecurity.AddToScheme(scheme)

	// Create fake policy object
	policy := &clientSecurity.AuthorizationPolicy{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "p"},
		Spec: apiSecurity.AuthorizationPolicy{
			Rules: []*apiSecurity.Rule{
				{
					From: []*apiSecurity.Rule_From{{Source: &apiSecurity.Source{RemoteIpBlocks: []string{"1.2.3.4"}}}},
					To:   []*apiSecurity.Rule_To{{Operation: &apiSecurity.Operation{Hosts: []string{"example.com"}}}},
				},
			},
		},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(policy).Build()
	r := &AuthPolicyWatcherReconciler{Client: fakeClient, Scheme: scheme}

	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "ns", Name: "p"}}
	_, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("Reconcile() error: %v", err)
	}
	if !called {
		t.Error("expected Slack alert but none was sent")
	}
	if !strings.HasPrefix(payload["text"], "AuthorizationPolicy ns/p 업데이트:") {
		t.Errorf("Slack payload text = %s; want prefix AuthorizationPolicy ns/p 업데이트:", payload["text"])
	}
}
