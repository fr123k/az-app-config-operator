package azure

import (
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/cucumber/godog/colors"
	"github.com/fr123k/aws-ssm-operator/api/v1alpha1"
	"github.com/spf13/pflag"
)

type Queue []string

func NewQueue() *Queue         { return &Queue{} }
func (q *Queue) Len() int      { return len(*q) }
func (q *Queue) Push(x string) { *q = append(*q, x) }

func (q *Queue) Pop() string {
	var el string
	el, *q = (*q)[0], (*q)[1:]
	return el
}

var goDogResponses = NewQueue()

var result map[string]string
var val string
var params = make(map[string]string)

func TestMain(m *testing.M) {
	flag.Parse()
	pflag.Parse()
	opts := godog.Options{
		Output:    colors.Colored(os.Stdout),
		Format:    "pretty",
		Paths:     []string{"features"},
		Randomize: time.Now().UTC().UnixNano(), // randomize scenario execution order
	}

	status := m.Run()

	status = status | godog.TestSuite{
		Name:                "godogs",
		ScenarioInitializer: InitializeScenario,
		TestSuiteInitializer: func(tsc *godog.TestSuiteContext) {
			server := AWSTestServer(goDogResponses)
			os.Setenv("LOCAL_STACK_ENDPOINT", server.URL)

			tsc.AfterSuite(func() {
				server.Close()
				os.Unsetenv("LOCAL_STACK_ENDPOINT")
			})
		},
		Options: &opts,
	}.Run()

	os.Exit(status)
}

// Given
func anExistingSsmParameterWithNameAndValue(name, value string) error {
	goDogResponses.Push(AppConfigParameter(name, value))
	return nil
}

func anExistingSsmParameterWithPathAndValue(path, value string) error {
	params[path] = value
	return nil
}

func Execute(arg interface{}) error {
	var err *SSMError
	appConfig, _ := NewAppClient(nil)
	switch a := arg.(type) {
	case v1alpha1.ParameterStoreRef:
		result, err = appConfig.SSMParameterValueToSecret(a)
	case []v1alpha1.ParametersStoreRef:
		result, _, err = appConfig.SSMParametersValueToSecret(a)
	default:
		return fmt.Errorf("I don't know about type %T!\n", a)
	}
	if err != nil {
		return err
	}
	return nil
}

// When
func theSsmParameterWithTheNameIsRetrieved(name string) error {
	return Execute(v1alpha1.ParameterStoreRef{Name: name})
}

func theSsmParameterWithThePathIsRetrieved(path string) error {
	goDogResponses.Push(AppConfigarameters(params))
	return Execute(v1alpha1.ParameterStoreRef{Path: path})
}

func theSsmParametersWithoutNameAreRetrieved() error {
	return Execute([]v1alpha1.ParametersStoreRef{{
		Key: "/user1/param1",
	}, {
		Key: "/user2/param2",
	},
	})
}

func theSsmParametersWithNameAreRetrieved() error {
	return Execute([]v1alpha1.ParametersStoreRef{{
		Name: "USER1",
		Key:  "/user1/param1",
	}, {
		Name: "USER2",
		Key:  "/user2/param2",
	},
	})
}

// Then
func theParameterResultShouldBeHaving() error {
	return nil
}

func theParameterValue(value string) error {
	if val != value {
		return fmt.Errorf("The value '%s' doesn't match expected value '%s", val, value)
	}
	return nil
}

func theParameterName(name string) error {
	var ok bool
	if val, ok = result[name]; !ok {
		return fmt.Errorf("Parameter %s not found", name)
	}
	return nil
}

func InitializeScenario(ctx *godog.ScenarioContext) {
	ctx.Step(`^An existing ssm parameter with name "([^"]*)" and value "([^"]*)"$`, anExistingSsmParameterWithNameAndValue)
	ctx.Step(`^An existing ssm parameter with path "([^"]*)" and value "([^"]*)"$`, anExistingSsmParameterWithPathAndValue)

	ctx.Step(`^the ssm parameter with the name "([^"]*)" is retrieved$`, theSsmParameterWithTheNameIsRetrieved)
	ctx.Step(`^the ssm parameter with the path "([^"]*)" is retrieved$`, theSsmParameterWithThePathIsRetrieved)
	ctx.Step(`^the ssm parameters without name are retrieved$`, theSsmParametersWithoutNameAreRetrieved)
	ctx.Step(`^the ssm parameters with name are retrieved$`, theSsmParametersWithNameAreRetrieved)

	ctx.Step(`^the parameter name "([^"]*)"$`, theParameterName)
	ctx.Step(`^the parameter result should be having$`, theParameterResultShouldBeHaving)
	ctx.Step(`^the parameter value "([^"]*)"$`, theParameterValue)
}
