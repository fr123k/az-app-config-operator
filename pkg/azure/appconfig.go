package azure

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azappconfig"
	"github.com/fr123k/aws-ssm-operator/api/v1alpha1"
	errs "github.com/pkg/errors"
)

type fakeCredentialResponse struct {
	token azcore.AccessToken
	err   error
}

type fakeCredential struct {
	getTokenCalls int
	mut           *sync.Mutex
	responses     []fakeCredentialResponse
	static        *fakeCredentialResponse
}

func NewFakeCredential() *fakeCredential {
	return &fakeCredential{mut: &sync.Mutex{}}
}

func (c *fakeCredential) SetResponse(tk azcore.AccessToken, err error) {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.static = &fakeCredentialResponse{tk, err}
}

func (c *fakeCredential) AppendResponse(tk azcore.AccessToken, err error) {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.responses = append(c.responses, fakeCredentialResponse{tk, err})
}

func (c *fakeCredential) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.getTokenCalls += 1
	if c.static != nil {
		return c.static.token, c.static.err
	}
	response := c.responses[0]
	c.responses = c.responses[1:]
	return response.token, response.err
}

type AppConfigClient struct {
	Client *azappconfig.Client
	ctx    context.Context
}

func endpoint(name *string) string {
	if lsEp := os.Getenv("LOCAL_STACK_ENDPOINT"); lsEp != "" {
		return lsEp
	}
	return fmt.Sprintf("https://%s.azconfig.io", *name)
}

func NewAppClient(name *string) (*AppConfigClient, error) {
	if lsEp := os.Getenv("LOCAL_STACK_ENDPOINT"); lsEp != "" {
		c2 := NewFakeCredential()
		c2.SetResponse(azcore.AccessToken{Token: "***", ExpiresOn: time.Now().Add(time.Hour)}, nil)

		client, err := azappconfig.NewClient(endpoint(name), c2, nil)
		if err != nil {
			return nil, err
		}
		ctx := context.TODO()
		return &AppConfigClient{Client: client, ctx: ctx}, err
	}
	credential, err := azidentity.NewDefaultAzureCredential(nil)

	if err != nil {
		return nil, err
	}

	client, err := azappconfig.NewClient(endpoint(name), credential, nil)
	if err != nil {
		return nil, err
	}
	ctx := context.TODO()
	return &AppConfigClient{Client: client, ctx: ctx}, err
}

// SSMParameterValueToSecret shapes fetched value so as to store them into K8S Secret
func (cli *AppConfigClient) SSMParameterValueToSecret(ref v1alpha1.ParameterStoreRef) (map[string]string, *SSMError) {
	if ref.Name != "" {
		return cli.Get(ref.Name)
	} else if ref.Path != "" {
		return cli.List(fmt.Sprintf("%s*", ref.Path))
	}
	return nil, NewSSMError("Invalid ParameterStoreRef provided atleast Name or Path has to be set.")
}

func (cli *AppConfigClient) Get(key string) (map[string]string, *SSMError) {

	resp, err := cli.Client.GetSetting(
		cli.ctx,
		key, nil)

	if err != nil {
		return nil, &SSMError{Err: err}
	}

	if resp.Key == nil {
		return nil, NewSSMError("Key not found")
	}

	return map[string]string{*resp.Key: *resp.Value}, nil
}

func (cli *AppConfigClient) List(key string) (map[string]string, *SSMError) {
	revPgr := cli.Client.NewListRevisionsPager(
		azappconfig.SettingSelector{
			KeyFilter: to.Ptr(key),
			Fields:    azappconfig.AllSettingFields(),
		},
		nil)

	m := make(map[string]string) // New empty set

	for revPgr.More() {
		if revResp, revErr := revPgr.NextPage(cli.ctx); revErr == nil {
			if revErr != nil {
				return nil, &SSMError{Err: revErr}
			}
			for _, setting := range revResp.Settings {
				if _, ok := m[*setting.Key]; ok {
					continue
				}
				ss := strings.Split(*setting.Key, "/")
				name := strings.ToUpper(ss[len(ss)-1])
				name = strings.ReplaceAll(name, "-", "_")
				m[name] = *setting.Value
			}
		}
	}
	return m, nil
}

func (cli *AppConfigClient) FetchParametersStoreValues(refs []v1alpha1.ParametersStoreRef) (map[string]string, map[string]string, *SSMError) {

	dict := make(map[string]string)
	anno := make(map[string]string)
	errors := make([]ParameterError, 0, len(refs))

	for _, ref := range refs {
		log.Info("fetching values from SSM Parameter Store", "Key", ref.Key, "Name", ref.Name)
		got, err := cli.Get(ref.Key)
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

func (cli *AppConfigClient) SSMParametersValueToSecret(ref []v1alpha1.ParametersStoreRef) (map[string]string, map[string]string, *SSMError) {
	params, anno, err := cli.FetchParametersStoreValues(ref)
	if err != nil {
		return nil, nil, err
	}
	if params == nil {
		return nil, nil, &SSMError{Err: errs.New("fetched value must not be nil")}
	}

	return params, anno, nil
}
