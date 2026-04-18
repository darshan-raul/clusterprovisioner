#!/bin/bash
set -e

cd orchestrator
pip install -r requirements.txt -t .
zip -r lambda.zip . -x "*.pyc" "__pycache__/*"
cd ..

cd start_codebuild
zip -r lambda.zip . -x "*.pyc" "__pycache__/*"
cd ..

echo "Lambdas packaged successfully."
