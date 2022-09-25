# az-app-config-operator

A Kubernetes operator that automatically maps what are stored in Azure Application Configutration Store into Kubernetes Secrets.

`az-app-config-operator` Custom Resources defines desired state of Kubernetes Secret fetched from SSM Parameter Store. Otherwise, `parameterstore-controller` controller monitors user's request and cached parameter values or credentials as plaintext into Kubernetes Secret.

## Before you begin

You have to store configuration parameter or credentials into Ayure Configuration Store.

Let's say, your application inside a Pod wish to connect to Aurora instances by using database user and password.

```bash
# Store database user with simple string
az appconfig kv set -n MyAppConfiguration \
    --key "/stg/foo-app/dbuser" \
    --value "dbuser" \
    --yes
# Store database password with simple string
az appconfig kv set -n MyAppConfiguration \
    --key "/stg/foo-app/dbpassword" \
    --value "dbpassword" \
    --yes
# Store database user with simple string
az appconfig kv set -n MyAppConfiguration \
    --key "/stg/foo-app/user/dbuser" \
    --value "dbuser" \
    --yes
# Store database password encrypted with simple string
az appconfig kv set -n MyAppConfiguration \
    --key "/stg/foo-app/password/dbpassword" \
    --value "dbpassword" \
    --yes
```

So, you can retrieve DB credentials by certain path like below:

```bash
$ az appconfig kv list -n MyAppConfiguration --key "/stg/foo-app/*"
{
    "Parameters": [
        {
            "Type": "String",
            "Name": "/stg/foo-app/dbuser",
            "Value": "foo-user"
        },
        {
            "Type": "SecureString",
            "Name": "/stg/foo-app/dbpassword",
            "Value": "*****"
        }
    ]
}
```

## Installation

```bash
# Setup Service Account
make deploy

# Verify that a Pod is running
kubectl get pod -l app=az-app-config-operator --watch -n aws-ssm
```

## Usage

Create an sample Paramter Store resource:

```bash
# Create an example Parameter Store resource by name
$ kubectl create -f example/parameterStoreRef/database-name.yaml

# Create an example Parameter Store resource by path
$ kubectl create -f example/parameterStoreRef/database-path.yaml

# Create an example Parameters Store resource by path
$ kubectl create -f example/parameterStoreRef/parameters.yaml
```

## Verifying

### Fetch Application Configuration Parameter by name

In case of using name reference, you can find your credentials in separate Secret resources as follows. The key to secret data is 'name' which is hardcoded and cannot be changed.

```bash
$ kubectl describe secret dbpassword
Name:         dbpassword
Namespace:    default
Labels:       app=dbpassword
Annotations:  <none>

Type:  Opaque

Data
====
name:  9 bytes

$ kubectl describe secret dbuser
Name:         dbuser
Namespace:    default
Labels:       app=dbuser
Annotations:  <none>

Type:  Opaque

Data
====
name:  7 bytes
```

You can reference secret data in your Deployment declaration as follows:
```yaml
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     name: sample-app
   spec:
     template:
       spec:
         containers:
           - image: toversus/sample-app
             env:
               - name: DB_USER
                 valueFrom:
                   secretKeyRef:
                     name: dbuser
                     key: name
               - name: DB_PASSWORD
                 valueFrom:
                   secretKeyRef:
                     name: dbpassword
                     key: name
```

### Fetch Application Configuration Parameters by path

In case of putting parameters together under certain path, you can find your credentials in single Secret resource as follows. The key to secret data is **last segment of path**. The following example shows that your credentials are located in `/stg/foo-app/dbuser` and `/stg/foo-app/dbpassword`, so you can retrieve them by `dbuser` and `dbpassword` keys respectively.

```bash
$ kubectl describe secret foo-app
Name:         foo-app
Namespace:    default
Labels:       app=foo-app
Annotations:  <none>

Type:  Opaque

Data
====
dbpassword:  10 bytes
dbuser:      6 bytes
```

You can reference secret data in your Deployment declaration as follows:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sample-app-pod
spec:
  containers:
  - name: sample-app
    image: toversus/sample-app
    envFrom:
    - secretRef:
        name: foo-app
  restartPolicy: Never
```
### Fetch Application Configuration by multiple paths

In case of putting parameters together from certain paths, you can find your credentials in single Secret resource as follows. The key to secret data is **full path**. The following example shows that your credentials are located in `/stg/foo-app/user/dbuser` and `/stg/foo-app/password/dbpassword`, so you can retrieve them by `dbuser` and `dbpassword` keys respectively.

```bash
$ kubectl describe secret foo-app-keys
Name:         foo-app-keys
Namespace:    default
Labels:       app=foo-app-keys
Annotations:  <none>

Type:  Opaque

Data
====
dbpassword:  10 bytes
dbuser:      7 bytes
```

You can reference secret data in your Deployment declaration as follows:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sample-app-pod
spec:
  containers:
  - name: sample-app
    image: toversus/sample-app
    envFrom:
    - secretRef:
        name: foo-app-keys
  restartPolicy: Never
```

## Clean up

To clean up all the components:

```bash
make undeploy
kubectl delete -f example/
```

## Build

```bash
  # enable docker buildkit engine
  export DOCKER_BUILDKIT=1
  make docker-build
```

## Local Development

### Prerequisites

#### Minikube

Installation for linux (Ubuntu)
```bash
  curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64
  sudo install minikube-linux-amd64 /usr/local/bin/minikube
```

### Minikube

```bash
  minikube start
```


### Azure Application Configuration Operator

```bash
  KUSTOMIZE_PROFILE=config/local make docker-build deploy
```

## Acknowledgements

The idea behind this project is fully based on [mumoshu/aws-secret-operator](https://github.com/mumoshu/aws-secret-operator). Thanks for your awesome work!
