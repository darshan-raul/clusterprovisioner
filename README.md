# EKS Cluster Control

On-demand EKS cluster provisioning via a Flutter Android app + AWS serverless backend.

**Stack:** Flutter (Android) · AWS API Gateway · AWS Lambda (Python 3.12) · Terraform · Amazon EKS

---

## Architecture

```
Flutter APK
    │  HTTPS + x-api-key header
    ▼
API Gateway (REST) — prod stage
    ├─► POST /cluster/start  ──► Lambda: cluster_control  ──► Terraform apply
    ├─► POST /cluster/stop   ──► Lambda: cluster_control  ──► Terraform destroy
    └─► GET  /cluster/status ──► Lambda: cluster_status   ──► EKS / CloudWatch

EventBridge (every 15 min)
    └─► Lambda: auto_teardown ──► checks uptime ──► Terraform destroy if > 4h

Terraform state: S3 bucket + DynamoDB lock table
CloudWatch Container Insights: EKS node CPU/memory metrics
```

---

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Flutter SDK | ≥ 3.3 | https://docs.flutter.dev/get-started/install |
| AWS CLI | v2 | https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2.html |
| Terraform | ≥ 1.6 | https://developer.hashicorp.com/terraform/install |
| Android SDK / device | API 21+ | Via Android Studio or `sdkmanager` |

Ensure `aws configure` is set up with your credentials and the `ap-south-1` region.

---

## Step 1 — Deploy Always-On Infrastructure (`infra/`)

The `infra/` Terraform creates:
- S3 bucket + DynamoDB table for EKS Terraform state
- All three Lambda functions (cluster_control, cluster_status, auto_teardown)
- API Gateway REST API with API key auth + throttling
- EventBridge rule (15-min auto-teardown check)
- IAM roles

> **Important — Terraform Lambda Layer:**  
> The `cluster_control` and `auto_teardown` Lambda functions need the **Terraform binary** at runtime (to run `terraform apply/destroy`).  
> You must build a Lambda Layer containing the Terraform Linux AMD64 binary before deploying.  
> See [Building the Terraform Lambda Layer](#building-the-terraform-lambda-layer) below.

```bash
cd infra/

# Initialize (uses local state for the always-on infra)
terraform init

# Preview
terraform plan

# Deploy
terraform apply
```

After apply, note the outputs:

```
api_base_url = "https://XXXXXXXXXX.execute-api.ap-south-1.amazonaws.com/prod"
api_key_id   = "xxxxxxxxxxxx"
```

Retrieve the actual API key value:
```bash
aws apigateway get-api-key --api-key <api_key_id> --include-value --query 'value' --output text
```

---

## Step 2 — Configure the Flutter App

Open `flutter_app/lib/services/api_service.dart` and replace the two placeholder constants:

```dart
static const _baseUrl = 'https://XXXXXXXXXX.execute-api.ap-south-1.amazonaws.com/prod';
static const _apiKey  = 'your-actual-api-key-value';
```

---

## Step 3 — Build and Install the Flutter APK

```bash
cd flutter_app/

# Fetch dependencies
flutter pub get

# Verify the app compiles
flutter analyze

# Build release APK
flutter build apk --release

# The APK is at:
# build/app/outputs/flutter-apk/app-release.apk

# Install directly on a connected Android device:
flutter install
```

> For a debug build during development: `flutter run`

---

## Step 4 — End-to-End Test

1. Open the app → you should see **STOPPED** status.
2. Tap **Start Cluster** → confirm → status changes to **PROVISIONING**.
3. Wait ~8-10 minutes (app auto-polls every 10 s).
4. Status changes to **RUNNING** → CPU/memory gauges appear.
5. Tap **Stop Cluster** → confirm → status changes to **DEPROVISIONING**.

---

## Building the Terraform Lambda Layer

The `cluster_control` and `auto_teardown` Lambdas shell out to the `terraform` binary. Package it as a Lambda Layer:

```bash
# Download Terraform Linux AMD64 binary (pin your version)
TERRAFORM_VERSION=1.8.4
wget https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_amd64.zip
unzip terraform_${TERRAFORM_VERSION}_linux_amd64.zip

# Lambda layers must be in a bin/ directory
mkdir -p terraform-layer/bin
cp terraform terraform-layer/bin/

# Zip the layer
cd terraform-layer
zip -r ../terraform-layer.zip .
cd ..

# Publish the layer
aws lambda publish-layer-version \
  --layer-name terraform-binary \
  --zip-file fileb://terraform-layer.zip \
  --compatible-runtimes python3.12 \
  --compatible-architectures x86_64
```

Copy the returned `LayerVersionArn` and uncomment the `layers` line in `infra/lambdas.tf`:

```hcl
layers = ["arn:aws:lambda:ap-south-1:ACCOUNT_ID:layer:terraform-binary:1"]
```

Then re-run `terraform apply` in `infra/`.

---

## Alternative: Async Terraform via CodeBuild

Lambda has a 15-minute hard timeout. EKS provisioning can take up to 12 minutes — it's risky. The recommended production approach is:

1. `cluster_control` Lambda triggers a **CodeBuild project** with the Terraform commands.
2. Lambda returns `{"status": "PROVISIONING"}` immediately.
3. Flutter polls `/cluster/status` every 10 s until EKS reports `ACTIVE`.

This avoids timeout races entirely. A CodeBuild run can last up to 8 hours.

---

## Testing Auto-Teardown

To test without waiting 4 hours:

```bash
# Temporarily override MAX_UPTIME_HOURS
aws lambda update-function-configuration \
  --function-name eks-auto-teardown \
  --environment 'Variables={MAX_UPTIME_HOURS=0}'

# Invoke manually
aws lambda invoke \
  --function-name eks-auto-teardown \
  --payload '{}' \
  /tmp/response.json

cat /tmp/response.json

# Restore the real value
aws lambda update-function-configuration \
  --function-name eks-auto-teardown \
  --environment 'Variables={MAX_UPTIME_HOURS=4}'
```

---

## Cost Estimate (ap-south-1)

| Resource | Rough cost |
|----------|-----------|
| EKS control plane | ~$0.10/hr |
| 1× t3.medium node | ~$0.034/hr |
| NAT gateway | ~$0.03/hr + data |
| **Total while running** | **~$0.16/hr** |
| Lambda + API Gateway + EventBridge | negligible |

With the 4-hour auto-teardown, a full cycle costs < $1.

---

## Known Limitations

- **Lambda timeout:** Terraform EKS apply takes 8-12 min. Lambda max is 15 min — tight. Use the CodeBuild alternative for safety.
- **API key in APK:** Low security — acceptable for a personal tool. Consider `flutter_secure_storage` to store it on device.
- **CloudWatch Container Insights:** Must be enabled. The `aws_eks_addon` in `terraform/eks.tf` handles this automatically.
- **Region:** Defaults to `ap-south-1` (Mumbai). Change `variable "region"` in both `terraform/variables.tf` and `infra/main.tf` to switch.
