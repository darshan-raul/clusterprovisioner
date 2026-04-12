import json
import os
import boto3
from datetime import datetime, timedelta, timezone
from botocore.exceptions import ClientError

CLUSTER_NAME = os.environ["CLUSTER_NAME"]
CLUSTER_START_TIME_PARAM = "/eks-control/cluster_start_time"

eks = boto3.client("eks")
cw = boto3.client("cloudwatch")
ssm = boto3.client("ssm")


def lambda_handler(event, context):
    try:
        cluster = eks.describe_cluster(name=CLUSTER_NAME)["cluster"]
        status = cluster["status"]  # ACTIVE, CREATING, DELETING, FAILED, etc.
        endpoint = cluster.get("endpoint", "")
        version = cluster.get("version", "")

        metrics = _get_metrics()
        uptime_minutes = _get_uptime_minutes()

        return _resp(
            200,
            _map_status(status),
            "OK",
            {
                "cluster_name": CLUSTER_NAME,
                "k8s_version": version,
                "endpoint": endpoint,
                "node_count": metrics.get("node_count", 0),
                "cpu_utilization": metrics.get("cpu", 0.0),
                "memory_utilization": metrics.get("memory", 0.0),
                "uptime_minutes": uptime_minutes,
            },
        )

    except ClientError as e:
        if e.response["Error"]["Code"] == "ResourceNotFoundException":
            return _resp(200, "STOPPED", "No cluster found", {})
        raise


def _get_metrics():
    """Query CloudWatch Container Insights for the cluster."""
    try:
        now = datetime.utcnow()
        response = cw.get_metric_statistics(
            Namespace="ContainerInsights",
            MetricName="node_cpu_utilization",
            Dimensions=[{"Name": "ClusterName", "Value": CLUSTER_NAME}],
            Period=60,
            Statistics=["Average"],
            StartTime=now - timedelta(minutes=5),
            EndTime=now,
        )
        datapoints = response.get("Datapoints", [])
        cpu = datapoints[-1]["Average"] if datapoints else 0.0

        mem_response = cw.get_metric_statistics(
            Namespace="ContainerInsights",
            MetricName="node_memory_utilization",
            Dimensions=[{"Name": "ClusterName", "Value": CLUSTER_NAME}],
            Period=60,
            Statistics=["Average"],
            StartTime=now - timedelta(minutes=5),
            EndTime=now,
        )
        mem_datapoints = mem_response.get("Datapoints", [])
        memory = mem_datapoints[-1]["Average"] if mem_datapoints else 0.0

        return {"cpu": round(cpu, 1), "memory": round(memory, 1), "node_count": 1}
    except Exception:
        return {"cpu": 0.0, "memory": 0.0, "node_count": 0}


def _get_uptime_minutes():
    """Read start time from SSM and compute elapsed minutes."""
    try:
        param = ssm.get_parameter(Name=CLUSTER_START_TIME_PARAM)
        start_time = datetime.fromisoformat(param["Parameter"]["Value"])
        elapsed = datetime.now(timezone.utc) - start_time
        return int(elapsed.total_seconds() / 60)
    except Exception:
        return 0


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
        "headers": {
            "Content-Type": "application/json",
            "Access-Control-Allow-Origin": "*",
        },
        "body": json.dumps({"status": status, "message": message, "data": data}),
    }
