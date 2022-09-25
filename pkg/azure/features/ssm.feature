Feature: Request single parameters by name or path from AWS SSM Parameter Store

    Scenario: Read one parameter by name

        Given An existing ssm parameter with name "param1" and value "value1"
        When the ssm parameter with the name "param1" is retrieved 
        Then the parameter result should be having
        And the parameter name "param1"
        And the parameter value "value1"

    Scenario: Read two parameters by path

        Given An existing ssm parameter with path "/user/param1" and value "value1"
        Given An existing ssm parameter with path "/user/param2" and value "value2"
        When the ssm parameter with the path "/user" is retrieved 
        Then the parameter result should be having
        And the parameter name "PARAM1"
        And the parameter value "value1"
        And the parameter name "PARAM2"
        And the parameter value "value2"
    
    Scenario: Read two parameters from different paths

        Given An existing ssm parameter with name "/user1/param1" and value "value1"
        Given An existing ssm parameter with name "/user2/param2" and value "value2"
        When the ssm parameters without name are retrieved 
        Then the parameter result should be having
        And the parameter name "PARAM1"
        And the parameter value "value1"
        And the parameter name "PARAM2"
        And the parameter value "value2"
    
    Scenario: Read two parameters from different paths and assign a name to each

        Given An existing ssm parameter with name "/user1/param1" and value "value1"
        Given An existing ssm parameter with name "/user2/param2" and value "value2"
        When the ssm parameters with name are retrieved 
        Then the parameter result should be having
        And the parameter name "USER1"
        And the parameter value "value1"
        And the parameter name "USER2"
        And the parameter value "value2"
