package azure

import (
	"bytes"
	_ "context"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/aws/aws-sdk-go-v2/config"
	"github.com/fr123k/aws-ssm-operator/api/v1alpha1"

	"github.com/stretchr/testify/assert"
)

var responses = NewQueue()

func AddSSMError(t *testing.T, code int, typ string, msg string) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(code)
		rw.Write([]byte(fmt.Sprintf(`{"__type":"%s", "message": "%s"}`, typ, msg)))
	}))
	// Close the server when test finishes
	t.Cleanup(server.Close)
	t.Setenv("LOCAL_STACK_ENDPOINT", server.URL)
}

func AWSTestServer(responses *Queue) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if responses.Len() < 1 {
			rw.WriteHeader(404)
			rw.Write([]byte(`{"__type":"Parameter not found", "message": "The parameter was not found"}`))
			return
		}
		response := responses.Pop()
		rw.Write([]byte(response))
	}))
	return server
}

func StartTestServer(t *testing.T) {
	server := AWSTestServer(responses)
	// Close the server when test finishes
	t.Cleanup(server.Close)
	t.Setenv("LOCAL_STACK_ENDPOINT", server.URL)
}

func AppConfigParameter(name string, value string) string {
	var b bytes.Buffer
	m := map[string]interface{}{"name": name, "value": value}

	t := template.Must(template.New("").Parse(`{
		"etag": "4f6dd610dd5e4deebc7fbaef685fb903",
		"key": "{{ .name }}",
		"label": "",
		"content_type": null,
		"value": "{{ .value }}",
		"last_modified": "2017-12-05T02:41:26+00:00",
		"locked": false,
		"tags": {
		  "t1": "value1",
		  "t2": "value2"
		}
	}`))

	err := t.Execute(&b, m)
	if err != nil {
		panic(err)
	}
	return b.String()
}

func SSMParameter(name string, value string) string {
	var b bytes.Buffer
	m := map[string]interface{}{"name": name, "value": value}

	t := template.Must(template.New("").Parse(`{
		"Parameter": {
			"ARN": "arn:aws:ssm:us-east-2:111122223333:parameter/{{ .name }}",
			"DataType": "text",
			"LastModifiedDate": 1582657288.8,
			"Name": "{{ .name }}",
			"Type": "SecureString",
			"Value": "{{ .value }}",
			"Version": 3
		}
	}`))
	err := t.Execute(&b, m)
	if err != nil {
		panic(err)
	}
	return b.String()
}

func jsonComma(m map[string]string) func() string {
	i := len(m)
	return func() string {
		i--
		if i == 0 {
			return ""
		}
		return ","
	}
}

func AppConfigarameters(values map[string]string) string {
	var b bytes.Buffer
	t := template.Must(template.New("").Funcs(template.FuncMap{"comma": jsonComma}).Parse(`{
		"items": [
{{$c := comma .}}
{{ range $k, $v := . }}
			{
				"etag": "4f6dd610dd5e4deebc7fbaef685fb903",
				"key": "{{ $k }}",
				"label": "",
				"content_type": null,
				"value": "{{ $v }}",
				"last_modified": "2017-12-05T02:41:26+00:00"
			}{{ call $c }}
{{ end }}
		]
		,"@nextLink": null
		}`))

	err := t.Execute(&b, values)
	if err != nil {
		panic(err)
	}
	return b.String()
}

func SSMParameters(values map[string]string) string {
	var b bytes.Buffer
	t := template.Must(template.New("").Funcs(template.FuncMap{"comma": jsonComma}).Parse(`{
		"Parameters": [
{{$c := comma .}}
{{ range $k, $v := . }}
		{
			"ARN": "arn:aws:ssm:us-east-2:111122223333:parameter/{{ $k }}",
			"DataType": "text",
			"LastModifiedDate": 1582657288.8,
			"Name": "{{ $k }}",
			"Type": "SecureString",
			"Value": "{{ $v }}",
			"Version": 3
		}{{ call $c }}
{{ end }}
		]
	}`))

	err := t.Execute(&b, values)
	if err != nil {
		panic(err)
	}
	return b.String()
}

// func TestAWSCfg(t *testing.T) {
// 	ssm := NewSSMClient(nil)
// 	defaultCfg, err := config.LoadDefaultConfig(context.TODO())
// 	assert.Nil(t, err)
// 	assert.Equal(t, defaultCfg.Region, ssm.cfg.Region)
// 	assert.Equal(t, defaultCfg.EndpointResolverWithOptions, ssm.cfg.EndpointResolverWithOptions)
// }

func TestSSMParameterValueToSecretByName(t *testing.T) {
	StartTestServer(t)
	responses.Push(AppConfigParameter("name", "aws-docs-example-parameter-value"))
	ssm, _ := NewAppClient(nil)
	result, err := ssm.SSMParameterValueToSecret(v1alpha1.ParameterStoreRef{Name: "name"})

	assert.Nil(t, err)
	assert.Equal(t, "aws-docs-example-parameter-value", result["name"])
}

func TestFetchParametersStoreValues(t *testing.T) {
	StartTestServer(t)
	responses.Push(AppConfigParameter("name", "aws-docs-example-parameter-value"))
	responses.Push(AppConfigParameter("name2", "an other parameter"))

	ssm, _ := NewAppClient(nil)

	result, anno, err := ssm.SSMParametersValueToSecret([]v1alpha1.ParametersStoreRef{{
		Name: "NAME",
		Key:  "name",
	}, {
		Key: "name2",
	}},
	)

	assert.Nil(t, err)
	assert.Len(t, anno, 0)
	assert.Len(t, result, 2)
	assert.Equal(t, "aws-docs-example-parameter-value", result["NAME"])
	assert.Equal(t, "an other parameter", result["NAME2"])
}

func TestSSMParameterValueToSecretByPath(t *testing.T) {
	StartTestServer(t)
	responses.Push(AppConfigarameters(map[string]string{"/path/param1": "aws-docs-example-parameter-value", "/path/param2": "value2"}))
	ssm, _ := NewAppClient(nil)

	result, err := ssm.SSMParameterValueToSecret(v1alpha1.ParameterStoreRef{Path: "path"})

	assert.Nil(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "aws-docs-example-parameter-value", result["PARAM1"])
	assert.Equal(t, "value2", result["PARAM2"])
}

// Error Cases

func TestOneNonExistingParameter(t *testing.T) {
	StartTestServer(t)
	responses.Push(AppConfigParameter("name", "aws-docs-example-parameter-value"))
	responses.Push(AppConfigParameter("name2", "an other parameter"))

	ssm, _ := NewAppClient(nil)

	result, anno, err := ssm.SSMParametersValueToSecret([]v1alpha1.ParametersStoreRef{{
		Name: "NAME",
		Key:  "name",
	}, {
		Name: "NAME2",
		Key:  "name2",
	}, {
		Name: "NAME3",
		Key:  "not_found",
	},
	},
	)

	assert.NotNil(t, err)
	assert.Len(t, err.ParameterErrors, 1)
	assert.Equal(t, "NAME3", err.ParameterErrors[0].Name)
	assert.Equal(t, "operation error SSM: GetParameter, https response error StatusCode: 400, RequestID: , api error Parameter not found: The parameter was not found", err.ParameterErrors[0].Err.Error())
	assert.Len(t, anno, 0)
	assert.Len(t, result, 0)
}

func TestInvalidParameterStoreRef(t *testing.T) {
	StartTestServer(t)
	ssm := NewSSMClient(nil)

	_, err := ssm.SSMParameterValueToSecret(v1alpha1.ParameterStoreRef{})

	assert.Equal(t, "Invalid ParameterStoreRef provided atleast Name or Path has to be set.", err.Error())
}

func TestSSMParameterValueToSecretByNotFoundPath(t *testing.T) {
	AddSSMError(t, 400, "ParameterNotFound", "the parameter path path not found")
	ssm := NewSSMClient(nil)

	_, err := ssm.SSMParameterValueToSecret(v1alpha1.ParameterStoreRef{Path: "path"})

	assert.Equal(t, "operation error SSM: GetParametersByPath, https response error StatusCode: 400, RequestID: , api error ParameterNotFound: the parameter path path not found", err.Error())
}
