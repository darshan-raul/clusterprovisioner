# EKS Cluster Control — Code Specification

**Project:** On-demand EKS cluster provisioning via Flutter Android app  
**Stack:** Flutter (Android) · AWS API Gateway · AWS Lambda (Python) · Terraform · Amazon EKS  
**Author prompt:** Anti Gravity internal tooling spec

---

## 1. Overview

A Flutter Android app acts as a remote control panel for an EKS cluster. The user can provision or deprovision a single-node EKS cluster on demand. When the cluster is running, the dashboard shows live metrics. A 4-hour auto-teardown guard prevents forgotten running clusters from accumulating cost.

---

## 2. Architecture Summary

```
Flutter APK
    │  HTTPS + API key header
    ▼
API Gateway (REST)
    ├─► POST /cluster/start  ──► Lambda: cluster_control.py  ──► Terraform apply
    ├─► POST /cluster/stop   ──► Lambda: cluster_control.py  ──► Terraform destroy
    └─► GET  /cluster/status ──► Lambda: cluster_status.py   ──► EKS / CloudWatch

EventBridge rule (every 15 min)
    └─► Lambda: auto_teardown.py ──► checks uptime ──► Terraform destroy if > 4h

Terraform state: S3 bucket + DynamoDB lock table
CloudWatch: EKS metrics + Lambda logs
```

---

## 3. Repository Structure

```
eks-control/
├── flutter_app/
│   ├── lib/
│   │   ├── main.dart
│   │   ├── screens/
│   │   │   └── dashboard_screen.dart
│   │   ├── widgets/
│   │   │   ├── cluster_status_card.dart
│   │   │   ├── metrics_panel.dart
│   │   │   └── action_buttons.dart
│   │   ├── services/
│   │   │   └── api_service.dart
│   │   └── models/
│   │       └── cluster_state.dart
│   └── pubspec.yaml
│
├── lambdas/
│   ├── cluster_control/
│   │   ├── handler.py
│   │   └── requirements.txt
│   ├── cluster_status/
│   │   ├── handler.py
│   │   └── requirements.txt
│   └── auto_teardown/
│       ├── handler.py
│       └── requirements.txt
│
├── terraform/
│   ├── main.tf
│   ├── variables.tf
│   ├── outputs.tf
│   ├── vpc.tf
│   ├── eks.tf
│   └── backend.tf
│
├── infra/
│   ├── api_gateway.tf
│   ├── lambdas.tf
│   ├── iam.tf
│   ├── eventbridge.tf
│   └── s3_state.tf
│
└── README.md
```

---

## 4. Authentication

Use **API Gateway API Keys** — no Cognito required.

- API Gateway resource policy restricts all unauthenticated calls.
- A single API key is generated at deploy time and stored in the Flutter app as a compile-time constant (or secure storage via `flutter_secure_storage`).
- Every request from Flutter includes the header: `x-api-key: <KEY>`.
- Usage plan on API Gateway: throttle to 10 req/min to prevent abuse.

> **Note:** For a personal tool this is sufficient. If you ever expose this more widely, migrate to Cognito or a short-lived token scheme.

---

## 5. API Gateway

### Base URL
`https://<id>.execute-api.<region>.amazonaws.com/prod`

### Endpoints

| Method | Path | Lambda | Description |
|--------|------|--------|-------------|
| POST | `/cluster/start` | `cluster_control` | Triggers `terraform apply` |
| POST | `/cluster/stop` | `cluster_control` | Triggers `terraform destroy` |
| GET | `/cluster/status` | `cluster_status` | Returns cluster state + metrics |

### Common Response Envelope

```json
{
  "status": "RUNNING | STOPPED | PROVISIONING | DEPROVISIONING | UNKNOWN",
  "message": "human-readable string",
  "data": { }
}
```

### CORS
Enable CORS on all endpoints. Origin: `*` is fine for a private APK.

---

## 6. Lambda Functions (Python 3.12)

### 6a. `cluster_control/handler.py`

Handles both start and stop by inspecting the request path.

```python
import json, os, subprocess, boto3, time
from datetime import datetime, timezone

TF_DIR = "/tmp/terraform"          # extracted from Lambda layer or S3
S3_STATE_BUCKET = os.environ["TF_STATE_BUCKET"]
DYNAMODB_LOCK_TABLE = os.environ["TF_LOCK_TABLE"]
CLUSTER_START_TIME_PARAM = "/eks-control/cluster_start_time"

ssm = boto3.client("ssm")

def lambda_handler(event, context):
    action = event["rawPath"].split("/")[-1]   # "start" or "stop"

    if action == "start":
        return _provision()
    elif action == "stop":
        return _deprovision()
    else:
        return _resp(400, "UNKNOWN", "Invalid action")


def _provision():
    _tf("init")
    _tf("apply", "-auto-approve")
    # Record start time in SSM Parameter Store for auto-teardown
    ssm.put_parameter(
        Name=CLUSTER_START_TIME_PARAM,
        Value=datetime.now(timezone.utc).isoformat(),
        Type="String",
        Overwrite=True
    )
    return _resp(200, "PROVISIONING", "Cluster provisioning started")


def _deprovision():
    _tf("init")
    _tf("destroy", "-auto-approve")
    ssm.delete_parameter(Name=CLUSTER_START_TIME_PARAM)
    return _resp(200, "DEPROVISIONING", "Cluster teardown started")


def _tf(*args):
    cmd = ["terraform", "-chdir=" + TF_DIR] + list(args)
    result = subprocess.run(cmd, capture_output=True, text=True, timeout=600)
    if result.returncode != 0:
        raise RuntimeError(f"Terraform failed: {result.stderr}")


def _resp(code, status, message, data=None):
    return {
        "statusCode": code,
        "headers": {"Content-Type": "application/json"},
        "body": json.dumps({"status": status, "message": message, "data": data or {}})
    }
```

**Environment Variables:**

| Key | Value |
|-----|-------|
| `TF_STATE_BUCKET` | S3 bucket name for Terraform state |
| `TF_LOCK_TABLE` | DynamoDB table name for state locking |
| `AWS_REGION` | Deployment region |

**Lambda config:**
- Runtime: Python 3.12
- Timeout: 15 minutes (max — Terraform apply can take ~8-10 min for EKS)
- Memory: 512 MB
- IAM role needs: `eks:*`, `ec2:*`, `iam:*`, `s3:*`, `dynamodb:*`, `ssm:PutParameter`

> **Important:** Terraform binary must be bundled as a Lambda Layer. Build a layer with the Terraform Linux AMD64 binary. Alternatively, trigger an ECS task or CodeBuild project to run Terraform — this avoids the Lambda timeout ceiling and is more robust for production. Document both options in README.

---

### 6b. `cluster_status/handler.py`

Polls EKS and CloudWatch for cluster state and metrics.

```python
import json, os, boto3
from botocore.exceptions import ClientError

CLUSTER_NAME = os.environ["CLUSTER_NAME"]
eks = boto3.client("eks")
cw = boto3.client("cloudwatch")

def lambda_handler(event, context):
    try:
        cluster = eks.describe_cluster(name=CLUSTER_NAME)["cluster"]
        status = cluster["status"]           # ACTIVE, CREATING, DELETING, etc.
        endpoint = cluster.get("endpoint", "")
        version = cluster.get("version", "")

        metrics = _get_metrics()

        return _resp(200, _map_status(status), "OK", {
            "cluster_name": CLUSTER_NAME,
            "k8s_version": version,
            "endpoint": endpoint,
            "node_count": metrics.get("node_count", 0),
            "cpu_utilization": metrics.get("cpu", 0.0),
            "memory_utilization": metrics.get("memory", 0.0),
            "uptime_minutes": metrics.get("uptime_minutes", 0),
        })

    except ClientError as e:
        if e.response["Error"]["Code"] == "ResourceNotFoundException":
            return _resp(200, "STOPPED", "No cluster found", {})
        raise


def _get_metrics():
    # Query CloudWatch Container Insights for the cluster
    # Returns CPU%, Memory%, node count
    # Simplified — expand with proper CW metric queries
    try:
        response = cw.get_metric_statistics(
            Namespace="ContainerInsights",
            MetricName="node_cpu_utilization",
            Dimensions=[{"Name": "ClusterName", "Value": CLUSTER_NAME}],
            Period=60,
            Statistics=["Average"],
            StartTime=__import__("datetime").datetime.utcnow() - __import__("datetime").timedelta(minutes=5),
            EndTime=__import__("datetime").datetime.utcnow(),
        )
        datapoints = response.get("Datapoints", [])
        cpu = datapoints[-1]["Average"] if datapoints else 0.0
        return {"cpu": round(cpu, 1), "memory": 0.0, "node_count": 1}
    except Exception:
        return {"cpu": 0.0, "memory": 0.0, "node_count": 0}


def _map_status(eks_status):
    return {
        "ACTIVE": "RUNNING",
        "CREATING": "PROVISIONING",
        "DELETING": "DEPROVISIONING",
        "FAILED": "ERROR",
    }.get(eks_status, "UNKNOWN")


def _resp(code, status, message, data):
    return {
        "statusCode": code,
        "headers": {"Content-Type": "application/json"},
        "body": json.dumps({"status": status, "message": message, "data": data})
    }
```

---

### 6c. `auto_teardown/handler.py`

Triggered by EventBridge every 15 minutes. Destroys the cluster if it has been running for more than 4 hours.

```python
import json, os, boto3, subprocess
from datetime import datetime, timezone, timedelta

CLUSTER_START_TIME_PARAM = "/eks-control/cluster_start_time"
MAX_UPTIME_HOURS = 4
TF_DIR = "/tmp/terraform"

ssm = boto3.client("ssm")

def lambda_handler(event, context):
    try:
        param = ssm.get_parameter(Name=CLUSTER_START_TIME_PARAM)
        start_time = datetime.fromisoformat(param["Parameter"]["Value"])
    except ssm.exceptions.ParameterNotFound:
        # No cluster running, nothing to do
        return {"status": "no_cluster"}

    elapsed = datetime.now(timezone.utc) - start_time
    if elapsed >= timedelta(hours=MAX_UPTIME_HOURS):
        print(f"Cluster has been up {elapsed}. Auto-tearing down.")
        _tf("init")
        _tf("destroy", "-auto-approve")
        ssm.delete_parameter(Name=CLUSTER_START_TIME_PARAM)
        return {"status": "torn_down", "uptime_hours": elapsed.total_seconds() / 3600}
    else:
        remaining = timedelta(hours=MAX_UPTIME_HOURS) - elapsed
        print(f"Cluster up {elapsed}. {remaining} remaining before auto-teardown.")
        return {"status": "ok", "remaining_minutes": remaining.total_seconds() / 60}


def _tf(*args):
    cmd = ["terraform", "-chdir=" + TF_DIR] + list(args)
    result = subprocess.run(cmd, capture_output=True, text=True, timeout=600)
    if result.returncode != 0:
        raise RuntimeError(f"Terraform failed: {result.stderr}")
```

**EventBridge Rule:**
- Schedule: `rate(15 minutes)`
- Target: `auto_teardown` Lambda
- Enable: always-on

---

## 7. Terraform — EKS Cluster

The Terraform config in `terraform/` provisions and deprovisions all cluster resources.

### `backend.tf`
```hcl
terraform {
  backend "s3" {
    bucket         = "eks-control-tf-state"        # must be pre-created
    key            = "eks-cluster/terraform.tfstate"
    region         = "ap-south-1"
    dynamodb_table = "eks-control-tf-lock"
    encrypt        = true
  }
}
```

### `vpc.tf`
```hcl
module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 5.0"

  name = "eks-control-vpc"
  cidr = "10.0.0.0/16"

  azs             = ["ap-south-1a", "ap-south-1b"]
  private_subnets = ["10.0.1.0/24", "10.0.2.0/24"]
  public_subnets  = ["10.0.101.0/24", "10.0.102.0/24"]

  enable_nat_gateway   = true
  single_nat_gateway   = true
  enable_dns_hostnames = true

  public_subnet_tags = {
    "kubernetes.io/role/elb" = "1"
  }
  private_subnet_tags = {
    "kubernetes.io/role/internal-elb" = "1"
  }
}
```

### `eks.tf`
```hcl
module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "~> 20.0"

  cluster_name    = var.cluster_name
  cluster_version = "1.30"

  vpc_id     = module.vpc.vpc_id
  subnet_ids = module.vpc.private_subnets

  cluster_endpoint_public_access = true

  eks_managed_node_groups = {
    default = {
      instance_types = ["t3.medium"]
      min_size       = 1
      max_size       = 1
      desired_size   = 1
    }
  }

  enable_cluster_creator_admin_permissions = true
}
```

### `variables.tf`
```hcl
variable "cluster_name" {
  default = "eks-on-demand"
}

variable "region" {
  default = "ap-south-1"
}
```

### `outputs.tf`
```hcl
output "cluster_name"     { value = module.eks.cluster_name }
output "cluster_endpoint" { value = module.eks.cluster_endpoint }
output "cluster_version"  { value = module.eks.cluster_version }
```

---

## 8. Flutter App

### `pubspec.yaml` dependencies
```yaml
dependencies:
  flutter:
    sdk: flutter
  http: ^1.2.0
  flutter_secure_storage: ^9.0.0
  provider: ^6.1.0
  fl_chart: ^0.68.0          # for CPU/memory gauge charts
  intl: ^0.19.0
```

### `lib/models/cluster_state.dart`
```dart
enum ClusterStatus {
  running,
  stopped,
  provisioning,
  deprovisioning,
  error,
  unknown,
}

class ClusterState {
  final ClusterStatus status;
  final String? clusterName;
  final String? k8sVersion;
  final int nodeCount;
  final double cpuUtilization;
  final double memoryUtilization;
  final int uptimeMinutes;

  const ClusterState({
    required this.status,
    this.clusterName,
    this.k8sVersion,
    this.nodeCount = 0,
    this.cpuUtilization = 0.0,
    this.memoryUtilization = 0.0,
    this.uptimeMinutes = 0,
  });

  factory ClusterState.fromJson(Map<String, dynamic> json) {
    final data = json['data'] as Map<String, dynamic>? ?? {};
    return ClusterState(
      status: _parseStatus(json['status'] as String),
      clusterName: data['cluster_name'] as String?,
      k8sVersion: data['k8s_version'] as String?,
      nodeCount: (data['node_count'] as num?)?.toInt() ?? 0,
      cpuUtilization: (data['cpu_utilization'] as num?)?.toDouble() ?? 0.0,
      memoryUtilization: (data['memory_utilization'] as num?)?.toDouble() ?? 0.0,
      uptimeMinutes: (data['uptime_minutes'] as num?)?.toInt() ?? 0,
    );
  }

  static ClusterStatus _parseStatus(String s) => switch (s) {
    'RUNNING'         => ClusterStatus.running,
    'STOPPED'         => ClusterStatus.stopped,
    'PROVISIONING'    => ClusterStatus.provisioning,
    'DEPROVISIONING'  => ClusterStatus.deprovisioning,
    'ERROR'           => ClusterStatus.error,
    _                 => ClusterStatus.unknown,
  };
}
```

### `lib/services/api_service.dart`
```dart
import 'dart:convert';
import 'package:http/http.dart' as http;

class ApiService {
  static const _baseUrl = 'https://<YOUR_API_ID>.execute-api.ap-south-1.amazonaws.com/prod';
  static const _apiKey  = '<YOUR_API_KEY>';  // or load from secure storage

  static Map<String, String> get _headers => {
    'x-api-key': _apiKey,
    'Content-Type': 'application/json',
  };

  Future<Map<String, dynamic>> getStatus() async {
    final res = await http.get(
      Uri.parse('$_baseUrl/cluster/status'),
      headers: _headers,
    );
    return json.decode(res.body) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> startCluster() async {
    final res = await http.post(
      Uri.parse('$_baseUrl/cluster/start'),
      headers: _headers,
    );
    return json.decode(res.body) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> stopCluster() async {
    final res = await http.post(
      Uri.parse('$_baseUrl/cluster/stop'),
      headers: _headers,
    );
    return json.decode(res.body) as Map<String, dynamic>;
  }
}
```

### `lib/screens/dashboard_screen.dart` — Key Structure

```dart
// Scaffold with:
// 1. AppBar: "EKS Control Panel"
// 2. Body:
//    ├── ClusterStatusCard   (status badge, cluster name, K8s version, uptime)
//    ├── MetricsPanel        (CPU gauge, Memory gauge, Node count) — visible only when RUNNING
//    └── ActionButtons       (Start Cluster / Stop Cluster) with loading + confirm dialog
// 3. FAB or pull-to-refresh triggers GET /cluster/status
// 4. Auto-poll status every 10 seconds when status is PROVISIONING or DEPROVISIONING

// Action button behavior:
// - "Start Cluster": disabled when status != STOPPED
// - "Stop Cluster":  disabled when status != RUNNING
// - Both show a confirmation dialog before sending request
// - Show SnackBar with message from API response
// - Set status to PROVISIONING/DEPROVISIONING optimistically while waiting for next poll
```

### Build APK
```bash
cd flutter_app
flutter build apk --release
# Output: build/app/outputs/flutter-apk/app-release.apk
```

---

## 9. IAM Roles

### Lambda Execution Role (cluster_control + auto_teardown)
```json
{
  "Version": "2012-10-17",
  "Statement": [
    { "Effect": "Allow", "Action": ["eks:*"],          "Resource": "*" },
    { "Effect": "Allow", "Action": ["ec2:*"],          "Resource": "*" },
    { "Effect": "Allow", "Action": ["iam:*"],          "Resource": "*" },
    { "Effect": "Allow", "Action": ["s3:*"],           "Resource": "arn:aws:s3:::eks-control-tf-state/*" },
    { "Effect": "Allow", "Action": ["dynamodb:*"],     "Resource": "arn:aws:dynamodb:::table/eks-control-tf-lock" },
    { "Effect": "Allow", "Action": ["ssm:PutParameter","ssm:GetParameter","ssm:DeleteParameter"], "Resource": "arn:aws:ssm:::parameter/eks-control/*" },
    { "Effect": "Allow", "Action": ["logs:*"],         "Resource": "*" }
  ]
}
```

### Lambda Execution Role (cluster_status)
```json
{
  "Statement": [
    { "Effect": "Allow", "Action": ["eks:DescribeCluster"], "Resource": "*" },
    { "Effect": "Allow", "Action": ["cloudwatch:GetMetricStatistics"], "Resource": "*" },
    { "Effect": "Allow", "Action": ["logs:*"], "Resource": "*" }
  ]
}
```

> Scope down the wildcard permissions for EC2 and IAM to specific resource ARN prefixes before any shared-account use.

---

## 10. EventBridge Auto-Teardown

```hcl
# infra/eventbridge.tf
resource "aws_cloudwatch_event_rule" "auto_teardown" {
  name                = "eks-auto-teardown"
  description         = "Check cluster uptime every 15 min and destroy if > 4 hours"
  schedule_expression = "rate(15 minutes)"
}

resource "aws_cloudwatch_event_target" "teardown_lambda" {
  rule      = aws_cloudwatch_event_rule.auto_teardown.name
  target_id = "AutoTeardownLambda"
  arn       = aws_lambda_function.auto_teardown.arn
}

resource "aws_lambda_permission" "allow_eventbridge" {
  statement_id  = "AllowEventBridge"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.auto_teardown.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.auto_teardown.arn
}
```

---

## 11. Pre-created Bootstrap Resources

These resources must exist **before** deploying anything (create once, never destroyed):

| Resource | Purpose |
|----------|---------|
| S3 bucket `eks-control-tf-state` | Terraform remote state |
| DynamoDB table `eks-control-tf-lock` | Terraform state locking |
| SSM Parameter `/eks-control/cluster_start_time` | Uptime tracking (created at runtime) |
| API Gateway API Key | Flutter auth header |

---

## 12. Deployment Order

1. Create S3 bucket and DynamoDB table manually (or via a bootstrap Terraform module with local state).
2. Deploy `infra/` Terraform (API Gateway, Lambdas, EventBridge, IAM) — this is always-on infrastructure.
3. Build and install the Flutter APK.
4. Test: hit "Start Cluster" → poll status → observe PROVISIONING → RUNNING.
5. Test auto-teardown: set `MAX_UPTIME_HOURS = 0` temporarily and trigger the EventBridge Lambda manually.

---

## 13. Known Limitations & Recommendations

- **Lambda timeout for Terraform:** EKS provisioning takes ~8-12 minutes. Lambda max timeout is 15 minutes — it is tight. Recommended alternative: have the Lambda kick off an async Step Functions state machine or a CodeBuild run for Terraform, then return immediately with `PROVISIONING`. Flutter polls `/cluster/status` every 10 seconds until it sees `RUNNING`.
- **Terraform binary in Lambda:** Bundle the Terraform binary as a Lambda Layer (Linux AMD64, ~80 MB compressed). Version-pin it.
- **API key in Flutter app:** Storing secrets in compiled APKs is low security. For personal use it's acceptable. Consider `flutter_secure_storage` to load it from device storage after first launch.
- **CloudWatch Container Insights:** Must be explicitly enabled on the EKS cluster. Add `aws_eks_addon` for `amazon-cloudwatch-observability` in `eks.tf`.
- **Region:** Spec defaults to `ap-south-1` (Mumbai) — closest to Pune. Adjust `variables.tf` if needed.