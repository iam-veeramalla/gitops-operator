package e2e

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"context"

	argoapi "github.com/argoproj-labs/argocd-operator/pkg/apis"
	argoapp "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	configv1 "github.com/openshift/api/config/v1"
	console "github.com/openshift/api/console/v1"
	routev1 "github.com/openshift/api/route/v1"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/types"

	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	"github.com/redhat-developer/gitops-operator/pkg/apis"
	operator "github.com/redhat-developer/gitops-operator/pkg/apis/pipelines/v1alpha1"
	"github.com/redhat-developer/gitops-operator/pkg/controller/gitopsservice"
)

var (
	retryInterval             = time.Second * 5
	timeout                   = time.Minute * 2
	cleanupRetryInterval      = time.Second * 1
	cleanupTimeout            = time.Second * 5
	insecure             bool = false
)

const (
	operatorName              = "gitops-operator"
	argoCDConfigMapName       = "argocd-cm"
	argoCDRouteName           = "openshift-gitops-server"
	argoCDNamespace           = "openshift-gitops"
	authURL                   = "/auth/realms/master/protocol/openid-connect/token"
	depracatedArgoCDNamespace = "openshift-pipelines-app-delivery"
	consoleLinkName           = "argocd"
	argoCDInstanceName        = "openshift-gitops"
	defaultKeycloakIdentifier = "keycloak"
	defaultTemplateIdentifier = "rhsso"
	realmURL                  = "/auth/admin/realms/argocd"
	rhssosecret               = "keycloak-secret"
)

func TestGitOpsService(t *testing.T) {
	err := framework.AddToFrameworkScheme(apis.AddToScheme, &operator.GitopsServiceList{})
	assertNoError(t, err)

	deployOperator(t)

	// run subtests
	t.Run("Validate kam service", validateKamService)
	t.Run("Validate GitOps Backend", validateGitOpsBackend)
	t.Run("Validate ConsoleLink", validateConsoleLink)
	t.Run("Validate ArgoCD Installation", validateArgoCDInstallation)
	t.Run("Validate ArgoCD Metrics Configuration", validateArgoCDMetrics)
	t.Run("Validate Redhat Single sign-on Installation", verifyRHSSOInstallation)
	t.Run("Validate Redhat Single sign-on Configuration", verifyRHSSOConfiguration)
	t.Run("Validate Redhat Single sign-on Uninstallation", verifyRHSSOUnInstallation)
	t.Run("Validate tear down of ArgoCD Installation", tearDownArgoCD)
}

func validateGitOpsBackend(t *testing.T) {
	framework.AddToFrameworkScheme(routev1.AddToScheme, &routev1.Route{})
	framework.AddToFrameworkScheme(configv1.AddToScheme, &configv1.ClusterVersion{})
	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()

	name := "cluster"
	f := framework.Global
	namespace, err := gitopsservice.GetBackendNamespace(f.Client.Client)
	assertNoError(t, err)

	// check backend deployment
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, name, 1, retryInterval, timeout)
	assertNoError(t, err)

	// check backend service
	err = f.Client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, &corev1.Service{})
	assertNoError(t, err)

	// check backend route
	err = f.Client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, &routev1.Route{})
	assertNoError(t, err)
}

func validateConsoleLink(t *testing.T) {
	framework.AddToFrameworkScheme(routev1.AddToScheme, &routev1.Route{})
	framework.AddToFrameworkScheme(console.AddToScheme, &console.ConsoleLink{})
	framework.AddToFrameworkScheme(configv1.AddToScheme, &configv1.ClusterVersion{})
	f := framework.Global

	route := &routev1.Route{}
	err := f.Client.Get(context.TODO(), types.NamespacedName{Name: argoCDRouteName, Namespace: argoCDNamespace}, route)
	assertNoError(t, err)

	// check ConsoleLink
	consoleLink := &console.ConsoleLink{}
	err = f.Client.Get(context.TODO(), types.NamespacedName{Name: consoleLinkName}, consoleLink)
	assertNoError(t, err)

	got := strings.TrimLeft(consoleLink.Spec.Href, "https://")
	if got != route.Spec.Host {
		t.Fatalf("Host mismatch: got %s, want %s", got, route.Spec.Host)
	}
}

func deployOperator(t *testing.T) {
	t.Helper()
	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()

	err := ctx.InitializeClusterResources(&framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	assertNoError(t, err)
	t.Log("Initialized Cluster resources")

	namespace, err := ctx.GetNamespace()
	assertNoError(t, err)

	err = e2eutil.WaitForOperatorDeployment(t, framework.Global.KubeClient, namespace, operatorName, 1, retryInterval, timeout)
	assertNoError(t, err)
}

func validateArgoCDInstallation(t *testing.T) {
	framework.AddToFrameworkScheme(argoapi.AddToScheme, &argoapp.ArgoCD{})
	framework.AddToFrameworkScheme(configv1.AddToScheme, &configv1.ClusterVersion{})

	f := framework.Global

	// Check if argocd namespace is created
	err := f.Client.Get(context.TODO(), types.NamespacedName{Name: argoCDNamespace}, &corev1.Namespace{})
	assertNoError(t, err)

	// Check if ArgoCD instance is created
	existingArgoInstance := &argoapp.ArgoCD{}
	err = f.Client.Get(context.TODO(), types.NamespacedName{Name: argoCDInstanceName, Namespace: argoCDNamespace}, existingArgoInstance)
	assertNoError(t, err)

	// modify the ArgoCD instance "manually"
	// and ensure that a manual modification of the
	// ArgoCD CR is allowed, and not overwritten
	// by the reconciler

	existingArgoInstance.Spec.DisableAdmin = true
	err = f.Client.Update(context.TODO(), existingArgoInstance)

	// assumption that an attempt to reconcile would have happened within 5 seconds.
	// This can definitely be improved.
	time.Sleep(5 * time.Second)

	// Check if ArgoCD CR was overwritten
	existingArgoInstance = &argoapp.ArgoCD{}
	err = f.Client.Get(context.TODO(), types.NamespacedName{Name: argoCDInstanceName, Namespace: argoCDNamespace}, existingArgoInstance)
	assertNoError(t, err)

	// check that this has not been overwritten
	assert.Equal(t, existingArgoInstance.Spec.DisableAdmin, true)

}

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func validateArgoCDMetrics(t *testing.T) {
	framework.AddToFrameworkScheme(rbacv1.AddToScheme, &rbacv1.Role{})
	framework.AddToFrameworkScheme(rbacv1.AddToScheme, &rbacv1.RoleBinding{})
	framework.AddToFrameworkScheme(monitoringv1.AddToScheme, &monitoringv1.ServiceMonitor{})
	framework.AddToFrameworkScheme(monitoringv1.AddToScheme, &monitoringv1.PrometheusRule{})

	f := framework.Global

	// Check the role was created
	role := rbacv1.Role{}
	readRoleName := fmt.Sprintf("%s-read", argoCDNamespace)
	err := f.Client.Get(context.TODO(),
		types.NamespacedName{Name: readRoleName, Namespace: argoCDNamespace}, &role)
	assertNoError(t, err)

	// Check the role binding was created
	roleBinding := rbacv1.RoleBinding{}
	roleBindingName := fmt.Sprintf("%s-prometheus-k8s-read-binding", argoCDNamespace)
	err = f.Client.Get(context.TODO(),
		types.NamespacedName{Name: roleBindingName, Namespace: argoCDNamespace},
		&roleBinding)
	assertNoError(t, err)

	// Check the application service monitor was created
	serviceMonitor := monitoringv1.ServiceMonitor{}
	serviceMonitorName := argoCDInstanceName
	err = f.Client.Get(context.TODO(),
		types.NamespacedName{Name: serviceMonitorName, Namespace: argoCDNamespace},
		&serviceMonitor)
	assertNoError(t, err)

	// Check the api server service monitor was created
	serviceMonitor = monitoringv1.ServiceMonitor{}
	serviceMonitorName = fmt.Sprintf("%s-server", argoCDInstanceName)
	err = f.Client.Get(context.TODO(),
		types.NamespacedName{Name: serviceMonitorName, Namespace: argoCDNamespace},
		&serviceMonitor)
	assertNoError(t, err)

	// Check the repo server service monitor was created
	serviceMonitor = monitoringv1.ServiceMonitor{}
	serviceMonitorName = fmt.Sprintf("%s-repo-server", argoCDInstanceName)
	err = f.Client.Get(context.TODO(),
		types.NamespacedName{Name: serviceMonitorName, Namespace: argoCDNamespace},
		&serviceMonitor)
	assertNoError(t, err)

	// Check the prometheus rule was created
	rule := monitoringv1.PrometheusRule{}
	err = f.Client.Get(context.TODO(),
		types.NamespacedName{Name: "gitops-operator-argocd-alerts", Namespace: argoCDNamespace},
		&rule)
	assertNoError(t, err)
}

func tearDownArgoCD(t *testing.T) {
	framework.AddToFrameworkScheme(argoapi.AddToScheme, &argoapp.ArgoCD{})
	framework.AddToFrameworkScheme(configv1.AddToScheme, &configv1.ClusterVersion{})

	f := framework.Global

	existingArgoInstance := &argoapp.ArgoCD{}
	err := f.Client.Get(context.TODO(), types.NamespacedName{Name: argoCDInstanceName, Namespace: argoCDNamespace}, existingArgoInstance)
	assertNoError(t, err)

	// Tear down Argo CD instance
	err = f.Client.Delete(context.TODO(), existingArgoInstance, &client.DeleteOptions{})
	assertNoError(t, err)

	err = e2eutil.WaitForDeletion(t, f.Client.Client, existingArgoInstance, retryInterval, timeout)
	assertNoError(t, err)

}
