import json
import os
import subprocess
import boto3
from datetime import datetime, timezone

TF_DIR = "/tmp/terraform"  # extracted from Lambda layer or S3
S3_STATE_BUCKET = os.environ["TF_STATE_BUCKET"]
DYNAMODB_LOCK_TABLE = os.environ["TF_LOCK_TABLE"]
CLUSTER_START_TIME_PARAM = "/eks-control/cluster_start_time"

ssm = boto3.client("ssm")


def lambda_handler(event, context):
    action = event["rawPath"].split("/")[-1]  # "start" or "stop"

    if action == "start":
        return _provision()
    elif action == "stop":
        return _deprovision()
    else:
        return _resp(400, "UNKNOWN", "Invalid action")


def _provision():
    try:
        _tf("init")
        _tf("apply", "-auto-approve")
        # Record start time in SSM Parameter Store for auto-teardown
        ssm.put_parameter(
            Name=CLUSTER_START_TIME_PARAM,
            Value=datetime.now(timezone.utc).isoformat(),
            Type="String",
            Overwrite=True,
        )
        return _resp(200, "PROVISIONING", "Cluster provisioning started")
    except RuntimeError as e:
        return _resp(500, "ERROR", str(e))


def _deprovision():
    try:
        _tf("init")
        _tf("destroy", "-auto-approve")
        try:
            ssm.delete_parameter(Name=CLUSTER_START_TIME_PARAM)
        except ssm.exceptions.ParameterNotFound:
            pass  # Already gone — that's fine
        return _resp(200, "DEPROVISIONING", "Cluster teardown started")
    except RuntimeError as e:
        return _resp(500, "ERROR", str(e))


def _tf(*args):
    cmd = ["terraform", "-chdir=" + TF_DIR] + list(args)
    result = subprocess.run(cmd, capture_output=True, text=True, timeout=600)
    if result.returncode != 0:
        raise RuntimeError(f"Terraform failed: {result.stderr}")


def _resp(code, status, message, data=None):
    return {
        "statusCode": code,
        "headers": {
            "Content-Type": "application/json",
            "Access-Control-Allow-Origin": "*",
        },
        "body": json.dumps(
            {"status": status, "message": message, "data": data or {}}
        ),
    }
