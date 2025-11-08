# CA Injector for Kubernetes 💉

<div align="center">
    <a href="https://sonarcloud.io/summary/new_code?id=omidiyanto_k8s-ca-injector">
        <img src="https://sonarcloud.io/api/project_badges/measure?project=omidiyanto_k8s-ca-injector&metric=alert_status" alt="Quality Gate Status">
    </a>
    <br><br>
    <img src="https://img.shields.io/badge/kubernetes-blue?style=for-the-badge&logo=kubernetes&logoColor=white">
    <img src="https://img.shields.io/badge/docker-green.svg?style=for-the-badge&logo=docker&logoColor=black">
    <img src="https://img.shields.io/badge/helm-red?style=for-the-badge&logo=helm&logoColor=white">
</div>
<br>

<img width="3040" height="4129" alt="image" src="https://github.com/user-attachments/assets/275d965c-1c68-41fa-a7b9-5a3656669929" />

A Kubernetes Mutating Admission Webhook that automatically injects custom CA certificate bundles into your pods.

This project is designed to allow off-the-shelf deployments to run in clusters with custom Certificate Authorities (CAs) without needing to modify container images. Say goodbye to the manual process of `ADD yourca.crt ...` and `RUN update-ca-certificates` in your Dockerfiles\!

## 🎯 The Problem

When working in an internal environment (corporate or air-gapped), applications often need to communicate with other services secured by TLS certificates issued by an internal (custom) CA.

Traditionally, the solution is to rebuild every application image to include the custom CA file. This process is:

  - **Tedious and repetitive**: You have to do it for every single application.
  - **Slows down CI/CD**: Rebuilding an image just to add a file takes time.
  - **Difficult to maintain**: If the CA is updated, all images must be rebuilt and redeployed.

## ✨ The Solution

`ca-injector` solves this problem elegantly. As an admission webhook, it intercepts Pod creation requests and dynamically modifies them "on-the-fly" before they are persisted to etcd.

This means there's no need to alter your container images at all. Simply add an annotation, and everything works automatically.

-----

## ⚙️ How It Works

The webhook performs three key actions when a pod with the correct annotation is created:

1.  **Adds a Volume**: Creates a volume within the Pod sourced from a Kubernetes **Secret**. This Secret contains your CA certificate bundle (`ca.crt`).
2.  **Mounts the Volume**: Mounts the volume into *every container* within the Pod at a predefined location (e.g., `/ssl/ca.crt`).
3.  **Sets an Environment Variable**: Adds the `SSL_CERT_FILE` and the `REQUESTS_CA_BUNDLE` environment variable to *every container*, pointing to the location of the newly mounted CA file. This variable is widely supported by TLS libraries like OpenSSL, Go, Python, Rust, C#, Dotnet, and more, causing your application to automatically trust the custom CA.

-----

## 📋 Prerequisites

Before you begin, ensure your environment meets the following requirements:

  - An active **Kubernetes cluster**.
  - **cert-manager** must be installed and running. This webhook will be deployed to the `cert-manager` namespace.
  - **trust-manager** must be installed. We'll use it to create the CA bundle and sync it into a Secret.
    ```bash
    helm repo add jetstack https://charts.jetstack.io --force-update

    helm install \
      cert-manager jetstack/cert-manager \
      --namespace cert-manager \
      --create-namespace \
      --version v1.19.1 \
      --set crds.enabled=true

    helm upgrade trust-manager jetstack/trust-manager \
      --install \
      --namespace cert-manager \
      --wait \
      --set secretTargets.enabled=true \
      --set secretTargets.authorizedSecretsAll=true
    ```
-----

## 🚀 Installation & Setup

The installation process consists of two main steps.
### Step 1: Install the k8s-ca-injector Using Helm Charts
Install using helm charts:
```bash
helm repo add helm repo add k8s-ca-injector https://omidiyanto.github.io/k8s-ca-injector/
helm repo update
helm install k8s-ca-injector k8s-ca-injector/k8s-ca-injector --version 0.1.0 --namespace cert-manager
```

### Step 2: Create the CA Bundle Secret

Use the `Bundle` CRD from **trust-manager** to define your CA sources and target them for creation as a Secret. This Secret is what `ca-injector` will read from.

Create a file named `ca-bundle.yaml`:

```yaml
apiVersion: trust.cert-manager.io/v1alpha1
kind: Bundle
metadata:
  # This name will be referenced in your pod's annotation
  name: my-root-ca
spec:
  sources:
    # (Optional) Include the system's default CAs
    - useDefaultCAs: true
    # Paste your custom CA certificate here
    - inLine: |
        -----BEGIN CERTIFICATE-----
        MIIDNTC..................
        -----END CERTIFICATE-----
  target:
    # Configuration to create a Secret
    secret:
      # The key name within the Secret that will hold the CA bundle
      key: "ca.crt"
```

> **Note**: By Using `Bundle`, `Trust Manager` will distribute it across all namespace as secret automatically.
-----

## 💡 How to Use

To enable the injection, you need to add annotations to your Pod spec. `ca-injector` provides several annotations for flexibility.

### Available Annotations

| Annotation | Type | Description |
| :--- | :--- | :--- |
| **`k8s/injectssl`** | **Required** | Specifies the name of the **Secret** containing the `ca.crt` you want to inject. This is the main trigger for the webhook. |
| `injectssl.k8s/mount-path` | Optional | Overrides the **directory path** inside the container where the certificate will be mounted. **Default**: `/ssl`. |
| `injectssl.k8s/extra-envs` | Optional | Adds custom environment variable names whose value will be set to the certificate file path. Separate multiple variable names with a comma (`,`). |

To inject the CA bundle into a Pod, simply add the `k8s/injectssl` annotation to the `spec.template.metadata.annotations` of your **Deployment**, **StatefulSet**, or other workload resource.

The value of the annotation must match the name of the `Bundle` (and the resulting Secret) that you created in `Step 2`.

### Example: Deployment

Here is a simple `Deployment` example using `curl` that will be injected with the `my-root-ca` bundle.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: my-namespace
spec:
  replicas: 1
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
      annotations:
        # 👇 THE MAGIC ANNOTATION IS HERE 👇
        k8s/injectssl: "my-root-ca"
    spec:
      containers:
      - name: my-app
        image: nginx:alpine
        # This container can now curl internal services with custom TLS
        command: ["sleep", "infinity"]
```

After you apply this `Deployment`, `ca-injector` will automatically modify the created Pods to include your CA certificate.

### Verifying the Injection

You can verify that the injection was successful by shelling into the Pod and checking for the file and environment variable.

```bash
# 1. Get the pod name
POD_NAME=$(kubectl get pods -n my-namespace -l app=my-app -o jsonpath='{.items[0].metadata.name}')

# 2. Describe the pod
kubectl describe $POD_NAME -n my-namespace -o jsonpath='{range .spec.containers[*]}Container: {.name}{"\n"}Environment:{range .env[*]}{"\n  "}{.name}: {.value}{end}{"\n\nMounts:"}{range .volumeMounts[*]}{"\n  "}{.mountPath} from {.name} (ro={.readOnly}){end}{"\n---\n"}{end}Volumes:{range .spec.volumes[*]}{"\n  "}{.name}: {.type}{end}{"\n"}'
```
```
Container: my-app
Environment:
  SSL_CERT_FILE: /ssl/ca.crt
  REQUESTS_CA_BUNDLE: /ssl/ca.crt
  NODE_EXTRA_CA_CERTS: /ssl/ca.crt

Mounts:
  /var/run/secrets/kubernetes.io/serviceaccount from kube-api-access-wzwbp (ro=true)
  /ssl from k8s-injected-ssl (ro=true)

Volumes:
  kube-api-access-wzwbp:
  k8s-injected-ssl:
```
As expected, the CA has been inject to the pods, the proof is the environment variable and volume is exists in the pod

```bash
# 3. Inside the pod's shell, check the variable and file
# Check the environment variable
kubectl exec -it $POD_NAME -n my-namespace -- cat /ssl/ca.crt
# You can even view its contents
-----BEGIN CERTIFICATE-----
MIIDNTC..................
-----END CERTIFICATE-----
```

If the commands above produce the expected output, the injection was successful\! 🎉

## 🧩 How to Contribute

We ❤️ contributions from the community!
If you'd like to improve **k8s-ca-injector**, add features, or fix bugs, here’s how you can get started:

1. **Fork** this repository.
2. **Clone** your fork locally:

   ```bash
   git clone https://github.com/<your-username>/k8s-ca-injector.git
   cd k8s-ca-injector
   ```
3. **Create a feature branch**:

   ```bash
   git checkout -b feature/my-new-feature
   ```
4. **Make your changes**, and ensure the code passes lint/tests.
5. **Commit and push**:

   ```bash
   git commit -m "feat: add awesome feature"
   git push origin feature/my-new-feature
   ```
6. **Open a Pull Request (PR)** to the `master` branch.

Please make sure your PR includes:

* Clear description of what it changes or fixes
* Tests or examples (if applicable)
* Updated documentation (if relevant)

> Contributions of all kinds are welcome — code, documentation, tests, or even ideas!
> Let’s make `k8s-ca-injector` better together 🚀
