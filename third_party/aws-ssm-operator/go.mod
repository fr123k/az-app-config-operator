module github.com/fr123k/aws-ssm-operator

go 1.26.0

require (
	github.com/aws/aws-sdk-go-v2 v1.43.0
	github.com/aws/aws-sdk-go-v2/config v1.32.31
	github.com/aws/aws-sdk-go-v2/credentials v1.19.30
	github.com/aws/aws-sdk-go-v2/service/ssm v1.73.0
	k8s.io/api v0.36.3
	k8s.io/apimachinery v0.36.3
	k8s.io/client-go v0.36.3
	sigs.k8s.io/controller-runtime v0.24.1
)

require github.com/aws/aws-sdk-go-v2/credentials v1.19.30