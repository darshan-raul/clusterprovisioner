import json
import os
import boto3

codebuild = boto3.client('codebuild')

def lambda_handler(event, context):
    try:
        task_token = event.get('task_token')
        action = event.get('action')
        cluster_id = event.get('cluster_id')
        user_id = event.get('user_id')
        aws_account_id = event.get('aws_account_id')
        region = event.get('region')
        instance_type = event.get('instance_type')
        cluster_name = event.get('name')
        
        project_name = os.environ.get('CODEBUILD_PROJECT')

        env_overrides = [
            {"name": "CLUSTER_ID", "value": cluster_id, "type": "PLAINTEXT"},
            {"name": "USER_ID", "value": user_id, "type": "PLAINTEXT"},
            {"name": "AWS_ACCOUNT_ID", "value": aws_account_id, "type": "PLAINTEXT"},
            {"name": "REGION", "value": region, "type": "PLAINTEXT"},
            {"name": "INSTANCE_TYPE", "value": instance_type, "type": "PLAINTEXT"},
            {"name": "CLUSTER_NAME", "value": cluster_name, "type": "PLAINTEXT"},
            {"name": "TASK_TOKEN", "value": task_token, "type": "PLAINTEXT"},
        ]

        # Start the build
        response = codebuild.start_build(
            projectName=project_name,
            environmentVariablesOverride=env_overrides
        )
        
        build_id = response['build']['id']
        print(f"Started CodeBuild for cluster {cluster_id}: {build_id}")
        
        return {
             "statusCode": 200,
             "body": json.dumps({"message": "CodeBuild started", "build_id": build_id})
        }

    except Exception as e:
        print(f"Error starting codebuild: {str(e)}")
        import traceback
        traceback.print_exc()
        
        # If we have a task token, fail the Step Functions task
        if 'task_token' in locals() and task_token:
            sfn = boto3.client('stepfunctions')
            try:
                sfn.send_task_failure(
                    taskToken=task_token,
                    error="StartCodeBuildError",
                    cause=str(e)
                )
            except Exception as sfn_e:
                print(f"Failed to send task failure to SFN: {str(sfn_e)}")
                
        raise e
