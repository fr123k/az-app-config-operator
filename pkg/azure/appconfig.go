package azure

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azappconfig"
	"github.com/fr123k/aws-ssm-operator/api/v1alpha1"
	errs "github.com/pkg/errors"
)

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
		// For local testing the endpoint is an HTTP test server. NewClient uses
		// a bearer-token policy that rejects authenticated requests over HTTP
		// (azcore >= v1.18), so use NewClientFromConnectionString (HMAC auth)
		// which has no such restriction.
		connStr := fmt.Sprintf("Endpoint=%s;Id=test;Secret=%s", lsEp, base64.StdEncoding.EncodeToString([]byte("test")))
		client, err := azappconfig.NewClientFromConnectionString(connStr, nil)
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
		revResp, revErr := revPgr.NextPage(cli.ctx)
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
