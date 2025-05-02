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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	securityv1beta1 "istio.io/client-go/pkg/apis/security/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// AuthPolicyWatcherReconciler reconciles a AuthPolicyWatcher object
type AuthPolicyWatcherReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=security.musinsa.com,resources=authpolicywatchers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=security.musinsa.com,resources=authpolicywatchers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=security.musinsa.com,resources=authpolicywatchers/finalizers,verbs=update
// +kubebuilder:rbac:groups=security.istio.io,resources=authorizationpolicies,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the AuthPolicyWatcher object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
// Kubernetes API 서버 → (이벤트 발생) → 컨트롤러(SetupWithManager) → 필터링(predicate) → Reconcile 메서드 실행
func (r *AuthPolicyWatcherReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    logger := log.FromContext(ctx)
    policy := &securityv1beta1.AuthorizationPolicy{}
    err := r.Get(ctx, req.NamespacedName, policy)
    
    // 삭제 이벤트 감지 (404 Not Found)
    if client.IgnoreNotFound(err) != nil {
        return ctrl.Result{}, err
    }
    
    // 객체가 존재하지 않으면 삭제된 것
    if err != nil {
        message := fmt.Sprintf("AuthorizationPolicy %s/%s 삭제됨", req.Namespace, req.Name)
        if err := sendSlackAlert(message); err != nil {
            logger.Error(err, "failed to send Slack alert for deletion")
        }
        return ctrl.Result{}, nil
    }
    
    // 기존 코드 (생성/수정 이벤트 처리)
    message := buildAlertMessage(policy)
    if err := sendSlackAlert(message); err != nil {
        logger.Error(err, "failed to send Slack alert")
    }
    
    return ctrl.Result{}, nil
}

func buildAlertMessage(policy *securityv1beta1.AuthorizationPolicy) string {
	ips := collectIPs(policy)
	hosts := collectHosts(policy)
	return fmt.Sprintf("AuthorizationPolicy %s/%s 업데이트: IPBlocks=%v, Hosts=%v", policy.Namespace, policy.Name, ips, hosts)
}

func sendSlackAlert(message string) error {
	webhook := os.Getenv("SLACK_WEBHOOK_URL")
	if webhook == "" {
		return fmt.Errorf("SLACK_WEBHOOK_URL 환경변수가 설정되어 있지 않습니다")
	}
	payload := map[string]string{"text": message}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(webhook, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("슬랙 웹훅 호출 실패: %s", resp.Status)
	}
	return nil
}

func collectIPs(policy *securityv1beta1.AuthorizationPolicy) []string {
	var ips []string
	for _, rule := range policy.Spec.Rules {
		for _, from := range rule.From {
			ips = append(ips, from.GetSource().GetRemoteIpBlocks()...)
		}
	}
	return ips
}

func collectHosts(policy *securityv1beta1.AuthorizationPolicy) []string {
	var hosts []string
	for _, rule := range policy.Spec.Rules {
		for _, to := range rule.To {
			hosts = append(hosts, to.GetOperation().GetHosts()...)
		}
	}
	return hosts
}

func contains(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *AuthPolicyWatcherReconciler) SetupWithManager(mgr ctrl.Manager) error {
	pred := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			policy := e.Object.(*securityv1beta1.AuthorizationPolicy)
			for _, rule := range policy.Spec.Rules {
				for _, from := range rule.From {
					if len(from.GetSource().GetRemoteIpBlocks()) > 0 {
						return true
					}
				}
				for _, to := range rule.To {
					if len(to.GetOperation().GetHosts()) > 0 {
						return true
					}
				}
			}
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldPolicy := e.ObjectOld.(*securityv1beta1.AuthorizationPolicy)
			newPolicy := e.ObjectNew.(*securityv1beta1.AuthorizationPolicy)
			oldIPs := collectIPs(oldPolicy)
			newIPs := collectIPs(newPolicy)
			for _, ip := range newIPs {
				if !contains(oldIPs, ip) {
					return true
				}
			}
			oldHosts := collectHosts(oldPolicy)
			newHosts := collectHosts(newPolicy)
			for _, host := range newHosts {
				if !contains(oldHosts, host) {
					return true
				}
			}
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return true
		},
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&securityv1beta1.AuthorizationPolicy{}).
		WithEventFilter(pred).
		Complete(r)
}
