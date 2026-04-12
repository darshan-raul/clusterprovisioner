import json
import os
import subprocess
import boto3
from datetime import datetime, timezone, timedelta

CLUSTER_START_TIME_PARAM = "/eks-control/cluster_start_time"
MAX_UPTIME_HOURS = int(os.environ.get("MAX_UPTIME_HOURS", "4"))
TF_DIR = "/tmp/terraform"

ssm = boto3.client("ssm")


def lambda_handler(event, context):
    try:
        param = ssm.get_parameter(Name=CLUSTER_START_TIME_PARAM)
        start_time = datetime.fromisoformat(param["Parameter"]["Value"])
    except ssm.exceptions.ParameterNotFound:
        # No cluster running — nothing to do
        print("No cluster_start_time parameter found. No cluster to tear down.")
        return {"status": "no_cluster"}

    elapsed = datetime.now(timezone.utc) - start_time

    if elapsed >= timedelta(hours=MAX_UPTIME_HOURS):
        print(f"Cluster has been up {elapsed}. Auto-tearing down.")
        try:
            _tf("init")
            _tf("destroy", "-auto-approve")
        except RuntimeError as e:
            print(f"Terraform destroy failed: {e}")
            return {"status": "error", "message": str(e)}

        try:
            ssm.delete_parameter(Name=CLUSTER_START_TIME_PARAM)
        except ssm.exceptions.ParameterNotFound:
            pass

        return {
            "status": "torn_down",
            "uptime_hours": round(elapsed.total_seconds() / 3600, 2),
        }
    else:
        remaining = timedelta(hours=MAX_UPTIME_HOURS) - elapsed
        print(
            f"Cluster up {elapsed}. {remaining} remaining before auto-teardown."
        )
        return {
            "status": "ok",
            "remaining_minutes": round(remaining.total_seconds() / 60, 1),
        }


def _tf(*args):
    cmd = ["terraform", "-chdir=" + TF_DIR] + list(args)
    result = subprocess.run(cmd, capture_output=True, text=True, timeout=600)
    if result.returncode != 0:
        raise RuntimeError(f"Terraform failed: {result.stderr}")
