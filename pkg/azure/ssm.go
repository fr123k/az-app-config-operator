package azure

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/fr123k/aws-ssm-operator/api/v1alpha1"

	errs "github.com/pkg/errors"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("parameterstore-controller")

// SSMClient preserves AWS config and SSM client itself
type SSMClient struct {
	cfg *aws.Config
	Ssm *ssm.Client
	ctx context.Context
}

func NewSSMClient(cfg *aws.Config) *SSMClient {
	ctx := context.TODO()
	if cfg == nil {
		cfg = AWSCfg(ctx)
	}
	ssm := ssm.NewFromConfig(*cfg)
	return &SSMClient{
		cfg: cfg,
		ctx: ctx,
		Ssm: ssm,
	}
}

func AWSCfg(ctx context.Context) *aws.Config {
	if lsEp := os.Getenv("LOCAL_STACK_ENDPOINT"); lsEp != "" {
		customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			if lsEp != "" {
				return aws.Endpoint{
					PartitionID:   "aws",
					URL:           lsEp,
					SigningRegion: "us-east-1",
				}, nil
			}

			// returning EndpointNotFoundError will allow the service to fallback to it's default resolution
			return aws.Endpoint{}, &aws.EndpointNotFoundError{}
		})
		log.Info("Setup localstack aws client")
		cfg := &aws.Config{
			Region:                      "us-east-1",
			Credentials:                 credentials.NewStaticCredentialsProvider("test", "test", ""),
			EndpointResolverWithOptions: customResolver,
		}
		return cfg
	}
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Error(err, "error loading default aws config")

	}
	return &cfg
}

// SSMParameterValueToSecret shapes fetched value so as to store them into K8S Secret
func (c *SSMClient) SSMParameterValueToSecret(ref v1alpha1.ParameterStoreRef) (map[string]string, *SSMError) {
	if ref.Name != "" {
		return c.GetParameterByName(ref.Name)
	} else if ref.Path != "" {
		return c.GetParameterByPath(ref.Path, ref.Recursive)
	}
	return nil, NewSSMError("Invalid ParameterStoreRef provided atleast Name or Path has to be set.")
}

func (c *SSMClient) FetchParametersStoreValues(refs []v1alpha1.ParametersStoreRef) (map[string]string, map[string]string, *SSMError) {

	dict := make(map[string]string)
	anno := make(map[string]string)
	errors := make([]ParameterError, 0, len(refs))

	for _, ref := range refs {
		log.Info("fetching values from SSM Parameter Store", "Key", ref.Key, "Name", ref.Name)
		got, err := c.GetParameterByName(ref.Key)
		if err != nil {
			log.Error(err, "error fetching values from SSM Parameter Store", "Key", ref.Key, "Name", ref.Name)
			anno[fmt.Sprintf("ssm.aws/%s_error", ref.Name)] = err.Error()
			errors = append(errors, ParameterError{Name: ref.Name, Err: err})
			continue
			// return nil, nil, err
		}
		name := ref.Name
		for k, v := range got {
			if name == "" {
				//TODO make this configurable in the ParameterStore crd
				ss := strings.Split(k, "/")
				name = strings.ToUpper(ss[len(ss)-1])
				name = strings.ReplaceAll(name, "-", "_")
			}
			dict[name] = v
		}
	}

	if len(errors) > 0 {
		return nil, nil, &SSMError{ParameterErrors: errors}
	}

	return dict, anno, nil
}

func (c *SSMClient) SSMParametersValueToSecret(ref []v1alpha1.ParametersStoreRef) (map[string]string, map[string]string, *SSMError) {
	params, anno, err := c.FetchParametersStoreValues(ref)
	if err != nil {
		return nil, nil, err
	}
	if params == nil {
		return nil, nil, &SSMError{Err: errs.New("fetched value must not be nil")}
	}

	return params, anno, nil
}

func (c *SSMClient) GetParameterByName(name string) (map[string]string, *SSMError) {
	log.Info("fetching values from SSM Parameter Store by name", "Name", name)
	got, err := c.Ssm.GetParameter(c.ctx, &ssm.GetParameterInput{
		Name:           &name,
		WithDecryption: true,
	})
	if err != nil {
		return nil, &SSMError{Err: err}
	}

	return map[string]string{*got.Parameter.Name: *got.Parameter.Value}, nil
}

func (c *SSMClient) GetParameterByPath(path string, recursive bool) (map[string]string, *SSMError) {
	log.Info("fetching values from SSM Parameter Store by path", "Path", path, "Recursive", recursive)
	page := ssm.NewGetParametersByPathPaginator(c.Ssm, &ssm.GetParametersByPathInput{
		Path:           &path,
		WithDecryption: true,
		Recursive:      recursive,
		MaxResults:     10,
	})
	dict := make(map[string]string)
	p := 0
	for p++; page.HasMorePages(); {
		got, err := page.NextPage(c.ctx)

		if err != nil {
			return nil, &SSMError{Err: err}
		}

		log.Info("fetching values from SSM Parameter Store by path", "Page", fmt.Sprintf("%d", p), "Retrieved Params", len(got.Parameters))

		for _, p := range got.Parameters {
			ss := strings.Split(*p.Name, "/")
			name := strings.ToUpper(ss[len(ss)-1])
			name = strings.ReplaceAll(name, "-", "_")
			dict[name] = *p.Value
		}
	}

	return dict, nil
}
