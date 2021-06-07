module github.com/redhat-developer/gitops-operator

go 1.16

require (
	github.com/argoproj-labs/argocd-operator v0.0.16-0.20210610090515-f6e29d689ae3
	github.com/bugsnag/bugsnag-go v1.5.3 // indirect
	github.com/bugsnag/panicwrap v1.2.0 // indirect
	github.com/coreos/prometheus-operator v0.40.0
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/garyburd/redigo v1.6.0 // indirect
	github.com/go-logr/logr v0.1.0
	github.com/gofrs/uuid v3.2.0+incompatible // indirect
	github.com/google/go-cmp v0.4.0
	github.com/gorilla/handlers v1.4.2 // indirect
	github.com/gorilla/mux v1.7.4 // indirect
	github.com/opencontainers/runc v1.0.0-rc9 // indirect
	github.com/openshift/api v3.9.1-0.20190916204813-cdbe64fb0c91+incompatible
	github.com/operator-framework/api v0.3.18
	github.com/operator-framework/operator-sdk v0.18.2
	github.com/rakyll/statik v0.1.7
	github.com/spf13/pflag v1.0.5
	github.com/yvasiyarov/go-metrics v0.0.0-20150112132944-c25f46c4b940 // indirect
	github.com/yvasiyarov/gorelic v0.0.7 // indirect
	github.com/yvasiyarov/newrelic_platform_go v0.0.0-20160601141957-9c099fbc30e9 // indirect
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.18.3
	k8s.io/apimachinery v0.18.3
	k8s.io/client-go v12.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.6.0
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by OLM
	github.com/argoproj-labs/argocd-operator => github.com/iam-veeramalla/argocd-operator v0.0.15-0.20210607112534-3f9f1314e45b
	k8s.io/client-go => k8s.io/client-go v0.18.2
)
