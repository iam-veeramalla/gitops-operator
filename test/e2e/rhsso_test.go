package e2e

import (
	"context"
	"crypto/tls"
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	argoapi "github.com/argoproj-labs/argocd-operator/pkg/apis"
	argoapp "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/common"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	appsv1 "github.com/openshift/api/apps/v1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	routev1 "github.com/openshift/api/route/v1"
	templatev1 "github.com/openshift/api/template/v1"
	"github.com/redhat-developer/gitops-operator/pkg/controller/gitopsservice"
	corev1 "k8s.io/api/core/v1"
)

type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	NotBeforePolicy  int    `json:"not-before-policy"`
	SessionState     string `json:"session_state"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func verifyRHSSOInstallation(t *testing.T) {
	framework.AddToFrameworkScheme(templatev1.AddToScheme, &templatev1.TemplateInstance{})
	framework.AddToFrameworkScheme(appsv1.AddToScheme, &appsv1.DeploymentConfig{})
	framework.AddToFrameworkScheme(oauthv1.AddToScheme, &oauthv1.OAuthClient{})
	framework.AddToFrameworkScheme(routev1.AddToScheme, &routev1.Route{})

	framework.AddToFrameworkScheme(argoapi.AddToScheme, &argoapp.ArgoCD{})

	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()

	f := framework.Global
	namespace, err := gitopsservice.GetBackendNamespace(f.Client.Client)
	assertNoError(t, err)

	// Update verifyTLS=false in the SSO configuration of ArgoCD resource.
	argocd := &argoapp.ArgoCD{}
	err = f.Client.Get(context.TODO(), types.NamespacedName{Name: argoCDInstanceName, Namespace: namespace}, argocd)
	assertNoError(t, err)

	argocd.Spec.SSO.VerifyTLS = &insecure
	err = f.Client.Update(context.TODO(), argocd)

	// Assumption that an attempt to reconcile would have happened within 5 seconds.
	time.Sleep(5 * time.Second)

	// Verify the creation of template instance.
	tInstance := &templatev1.TemplateInstance{}
	err = f.Client.Get(context.TODO(), types.NamespacedName{Name: defaultTemplateIdentifier, Namespace: namespace}, tInstance)
	assertNoError(t, err)

	// Verify the keycloak Deployment and availble replicas.
	dc := &appsv1.DeploymentConfig{}
	err = f.Client.Get(context.TODO(), types.NamespacedName{Name: defaultKeycloakIdentifier, Namespace: namespace}, dc)
	assertNoError(t, err)
	assert.Assert(t, dc.Status.AvailableReplicas == 1)

	// Verify the keycloak Deployment and availble replicas.
	svc := &corev1.Service{}
	err = f.Client.Get(context.TODO(), types.NamespacedName{Name: defaultKeycloakIdentifier, Namespace: namespace}, svc)
	assertNoError(t, err)

	// Verify the creation of route.
	route := &routev1.Route{}
	err = f.Client.Get(context.TODO(), types.NamespacedName{Name: defaultKeycloakIdentifier, Namespace: namespace}, route)
	assertNoError(t, err)
}

func verifyRHSSOConfiguration(t *testing.T) {
	framework.AddToFrameworkScheme(templatev1.AddToScheme, &templatev1.TemplateInstance{})
	framework.AddToFrameworkScheme(appsv1.AddToScheme, &appsv1.DeploymentConfig{})
	framework.AddToFrameworkScheme(oauthv1.AddToScheme, &oauthv1.OAuthClient{})
	framework.AddToFrameworkScheme(routev1.AddToScheme, &routev1.Route{})

	framework.AddToFrameworkScheme(argoapi.AddToScheme, &argoapp.ArgoCD{})

	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()

	f := framework.Global
	namespace, err := gitopsservice.GetBackendNamespace(f.Client.Client)
	assertNoError(t, err)

	// Verify OIDC Configuration is created.
	cm := &corev1.ConfigMap{}
	err = f.Client.Get(context.TODO(), types.NamespacedName{Name: argoCDConfigMapName, Namespace: namespace}, cm)
	assertNoError(t, err)
	assert.Assert(t, cm.Data[common.ArgoCDKeyOIDCConfig] != "")

	// Get keycloak URL and credentials.
	route := &routev1.Route{}
	err = f.Client.Get(context.TODO(), types.NamespacedName{Name: defaultKeycloakIdentifier, Namespace: namespace}, route)
	assertNoError(t, err)

	secret := &corev1.Secret{}
	err = f.Client.Get(context.TODO(), types.NamespacedName{Name: rhssosecret, Namespace: namespace}, secret)
	assertNoError(t, err)

	userEnc := b64.URLEncoding.EncodeToString(secret.Data["SSO_USERNAME"])
	user, _ := b64.URLEncoding.DecodeString(userEnc)

	passEnc := b64.URLEncoding.EncodeToString(secret.Data["SSO_PASSWORD"])
	pass, _ := b64.URLEncoding.DecodeString(passEnc)

	// Verify RHSSO Realm creation. If Realm is created, HTTP GET request should return 200.
	// Get Auth token
	accessURL := fmt.Sprintf("https://%s%s", route.Spec.Host, authURL)
	argoRealmURL := fmt.Sprintf("https://%s%s", route.Spec.Host, realmURL)

	accessToken, err := getAccessToken(string(user), string(pass), accessURL)
	assertNoError(t, err)

	client := http.Client{}
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	request, err := http.NewRequest("GET", argoRealmURL, nil)
	assertNoError(t, err)

	// Set headers.
	request.Header.Set("Content-Type", "application/json")
	request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	response, err := client.Do(request)
	assertNoError(t, err)
	defer response.Body.Close()

	// Verify response
	assert.Assert(t, response.StatusCode == http.StatusOK)

	b, err := ioutil.ReadAll(response.Body)
	assertNoError(t, err)

	m := make(map[string]interface{})
	err = json.Unmarshal(b, &m)

	assert.Assert(t, err)
	assert.Assert(t, m["realm"] == "argocd")
	assert.Assert(t, m["registrationFlow"] == "registration")
	assert.Assert(t, m["browserFlow"] == "browser")
	assert.Assert(t, m["clientAuthenticationFlow"] == "clients")
	assert.Assert(t, m["directGrantFlow"] == "direct grant")
	assert.Assert(t, m["loginWithEmailAllowed"] == true)

	idps := m["identityProviders"].([]interface{})
	idp := idps[0].(map[string]interface{})

	assert.Assert(t, idp["alias"] == "openshift-v4")
	assert.Assert(t, idp["displayName"] == "Login with OpenShift")
	assert.Assert(t, idp["providerId"] == "openshift-v4")
	assert.Assert(t, idp["firstBrokerLoginFlowAlias"] == "first broker login")
}

func verifyRHSSOUnInstallation(t *testing.T) {
	framework.AddToFrameworkScheme(templatev1.AddToScheme, &templatev1.TemplateInstance{})
	framework.AddToFrameworkScheme(argoapi.AddToScheme, &argoapp.ArgoCD{})

	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()

	f := framework.Global
	namespace, err := gitopsservice.GetBackendNamespace(f.Client.Client)
	assertNoError(t, err)

	argocd := &argoapp.ArgoCD{}
	err = f.Client.Get(context.TODO(), types.NamespacedName{Name: argoCDInstanceName, Namespace: namespace}, argocd)
	assertNoError(t, err)

	// Remove SSO feild from ArgoCD CR.
	argocd.Spec.SSO = nil
	err = f.Client.Update(context.TODO(), argocd)
	assertNoError(t, err)

	// Assumption that an attempt to reconcile would have happened within 10 seconds.
	time.Sleep(10 * time.Second)

	// Verify OIDC Configuration is removed.
	cm := &corev1.ConfigMap{}
	err = f.Client.Get(context.TODO(), types.NamespacedName{Name: argoCDConfigMapName, Namespace: namespace}, cm)
	assertNoError(t, err)
	assert.Assert(t, cm.Data[common.ArgoCDKeyOIDCConfig] == "")

	// Verify if the template instance is deleted.
	templateInstance := &templatev1.TemplateInstance{}
	err = f.Client.Get(context.TODO(), types.NamespacedName{Name: defaultTemplateIdentifier, Namespace: namespace}, templateInstance)
	assert.Equal(t, errors.IsNotFound(err), true)

	// Add SSO feild back and verify reconcilation.
	argocd.Spec.SSO = &argoapp.ArgoCDSSOSpec{
		Provider:  defaultKeycloakIdentifier,
		VerifyTLS: &insecure,
	}
	err = f.Client.Update(context.TODO(), argocd)
	assertNoError(t, err)

	// Assumption that an attempt to reconcile would have happened within 15 seconds.
	time.Sleep(15 * time.Second)

	// Verify if the template instance is created.
	templateInstance = &templatev1.TemplateInstance{}
	err = f.Client.Get(context.TODO(), types.NamespacedName{Name: defaultTemplateIdentifier, Namespace: namespace}, templateInstance)
	assertNoError(t, err)
}

func getAccessToken(user, pass, accessURL string) (string, error) {
	form := url.Values{}
	form.Add("username", user)
	form.Add("password", pass)
	form.Add("client_id", "admin-cli")
	form.Add("grant_type", "password")

	client := http.Client{}
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	req, err := http.NewRequest(
		"POST",
		accessURL,
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return "", err
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	tokenRes := &tokenResponse{}
	err = json.Unmarshal(body, tokenRes)
	if err != nil {
		return "", err
	}

	if tokenRes.Error != "" {
		return "", err
	}

	return tokenRes.AccessToken, nil
}
