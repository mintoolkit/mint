#!/bin/bash
# SPDX-FileCopyrightText: Copyright (c) 2025 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
# SPDX-License-Identifier: Apache-2.0
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Configuration
VLLM_IMAGE="${VLLM_IMAGE:-vllm/vllm-openai:v0.15.1}"
SLIM_TAG="${SLIM_TAG:-$(echo $VLLM_IMAGE | sed 's|.*/||; s/:/-slim:/')}"
MODEL="${MODEL:-TinyLlama/TinyLlama-1.1B-Chat-v1.0}"
MAX_WAIT_MINUTES="${MAX_WAIT_MINUTES:-20}"
CONTAINER_PORT=8000
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="${OUTPUT_DIR:-$SCRIPT_DIR}"
RESULTS_FILE="${OUTPUT_DIR}/vllm_test_results.json"
SLIM_RESULTS_FILE="${OUTPUT_DIR}/vllm_test_results_slim.json"

# Check for required HF_TOKEN
if [ -z "$HF_TOKEN" ]; then
    echo "Error: HF_TOKEN environment variable is not set"
    echo "Please set it to your Hugging Face access token"
    exit 1
fi

# Find the slim binary - check for mint or slim in PATH, or use local binary
if command -v mint &> /dev/null; then
    SLIM_CMD="mint"
elif command -v slim &> /dev/null; then
    SLIM_CMD="slim"
elif [ -x "${SCRIPT_DIR}/../../bin/linux/slim" ]; then
    SLIM_CMD="${SCRIPT_DIR}/../../bin/linux/slim"
else
    echo "Error: mint/slim binary not found"
    echo "Please install mint or ensure bin/linux/slim exists"
    exit 1
fi
echo "Using slim binary: $SLIM_CMD"

# Cleanup function
cleanup() {
    echo "Cleaning up..."
    if [ -n "$MONITOR_PID" ] && kill -0 "$MONITOR_PID" 2>/dev/null; then
        kill "$MONITOR_PID" 2>/dev/null
    fi
    if [ -n "$SLIM_PID" ] && kill -0 "$SLIM_PID" 2>/dev/null; then
        # Send SIGINT to mint to gracefully stop
        kill -INT "$SLIM_PID" 2>/dev/null
        wait "$SLIM_PID" 2>/dev/null
    fi
}

trap cleanup EXIT

# Create host config file with ulimit settings and capabilities
cat > host-config.json <<'EOF'
{
  "IpcMode": "host",
  "PidMode": "host",
  "CapAdd": ["SYS_ADMIN", "SYS_PTRACE"],
  "Ulimits": [
    {
      "Name": "memlock",
      "Soft": -1,
      "Hard": -1
    },
    {
      "Name": "stack",
      "Soft": 67108864,
      "Hard": 67108864
    },
    {
      "Name": "nofile",
      "Soft": 1048576,
      "Hard": 1048576
    }
  ]
}
EOF
MAX_SEQ_LEN=2048
echo "============================================="
echo "VLLM mint Test"
echo "============================================="
echo "Source Image: $VLLM_IMAGE"
echo "Slim Tag: $SLIM_TAG"
echo "Model: $MODEL"
echo "Max Wait: $MAX_WAIT_MINUTES minutes"
echo "MAX_SEQ_LEN: $MAX_SEQ_LEN"
echo "Results File: $RESULTS_FILE"
echo "HF_TOKEN: ${HF_TOKEN:0:3}..."
echo "============================================="

# Function to run mint with monitoring
run_slim_with_monitor() {
    local target_image="$1"
    local output_tag="$2"
    local results_file="$3"
    local is_original="$4"

    echo ""
    echo "Starting mint build for: $target_image"
    echo "Output tag: $output_tag"

    # Create a named pipe for signaling
    SIGNAL_PIPE=$(mktemp -u)
    mkfifo "$SIGNAL_PIPE"

    # Start mint in the background
    # Using --continue-after signal to allow the monitor to signal when done
    local mint_cmd=("$SLIM_CMD" build
        --target "$target_image"
        --tag "$output_tag"
        --cro-host-config-file host-config.json
        --cro-shm-size 1200
        --cro-device-request '{"Count":-1, "Capabilities":[["gpu"]]}'
        --cro-runtime nvidia
        --expose "${CONTAINER_PORT}"
        --publish-port "${CONTAINER_PORT}:${CONTAINER_PORT}"
        --publish-exposed-ports
        --env "HF_TOKEN=${HF_TOKEN}"
        --cmd "${MODEL} --max_model_len ${MAX_SEQ_LEN}"
        --http-probe=false
        --rta-source-ptrace=true
        --continue-after signal
        --preserve-path /etc/ld.so.conf
        --preserve-path /etc/ld.so.conf.d
        --exclude-pattern "/root/.cache/**"
        .)
    echo "Running: ${mint_cmd[*]}"
    "${mint_cmd[@]}" &

    SLIM_PID=$!
    echo "mint started with PID: $SLIM_PID"

    # Wait a moment for the container to start
    sleep 10

    # Start the monitor/test script in the background
    echo "Starting API monitor and test runner..."
    python3 "${SCRIPT_DIR}/vllm_api_tests.py" \
        --host "localhost" \
        --port "$CONTAINER_PORT" \
        --output "$results_file" \
        --max-wait "$MAX_WAIT_MINUTES" \
        --signal-pid "$SLIM_PID" &

    MONITOR_PID=$!
    echo "Monitor started with PID: $MONITOR_PID"

    # Wait for the monitor to complete
    wait "$MONITOR_PID"
    MONITOR_EXIT_CODE=$?
    echo "Monitor completed with exit code: $MONITOR_EXIT_CODE"

    # Wait for mint to complete
    wait "$SLIM_PID"
    SLIM_EXIT_CODE=$?
    echo "mint completed with exit code: $SLIM_EXIT_CODE"

    # Cleanup the signal pipe
    rm -f "$SIGNAL_PIPE"

    if [ $SLIM_EXIT_CODE -ne 0 ]; then
        echo "Warning: mint exited with code $SLIM_EXIT_CODE"
    fi

    return $MONITOR_EXIT_CODE
}

# Phase 1: Build slim image from original and run tests
echo ""
echo "============================================="
echo "Phase 1: Building slim image from original"
echo "============================================="

run_slim_with_monitor "$VLLM_IMAGE" "$SLIM_TAG" "$RESULTS_FILE" "true"
PHASE1_EXIT=$?

if [ ! -f "$RESULTS_FILE" ]; then
    echo "Error: Results file not created during Phase 1"
    exit 1
fi

echo ""
echo "Phase 1 Results:"
cat "$RESULTS_FILE"

# Phase 2: Run slim image and test it
echo ""
echo "============================================="
echo "Phase 2: Testing the slimmed image"
echo "============================================="

# Run the slimmed image directly (not through mint) and test it
echo "Starting slimmed container for testing..."
docker_run_cmd=(docker run -d
    --runtime nvidia
    --gpus all
    --ipc=host
    --ulimit memlock=-1
    --ulimit stack=67108864
    --shm-size=1200m
    -e HF_TOKEN
    -p "${CONTAINER_PORT}:${CONTAINER_PORT}"
    "$SLIM_TAG"
    "$MODEL" --max_model_len "$MAX_SEQ_LEN")
echo "Running: ${docker_run_cmd[*]}"
SLIM_CONTAINER_ID=$("${docker_run_cmd[@]}")

echo "Slim container started: $SLIM_CONTAINER_ID"

# Run tests against the slim container
python3 "${SCRIPT_DIR}/vllm_api_tests.py" \
    --host "localhost" \
    --port "$CONTAINER_PORT" \
    --output "$SLIM_RESULTS_FILE" \
    --max-wait "$MAX_WAIT_MINUTES"

PHASE2_EXIT=$?

# Stop the slim container
docker stop "$SLIM_CONTAINER_ID"
docker rm "$SLIM_CONTAINER_ID"

if [ ! -f "$SLIM_RESULTS_FILE" ]; then
    echo "Error: Slim results file not created during Phase 2"
    exit 1
fi

echo ""
echo "Phase 2 Results (Slim Image):"
cat "$SLIM_RESULTS_FILE"

# Compare results
echo ""
echo "============================================="
echo "Comparing Results"
echo "============================================="

python3 - "$RESULTS_FILE" "$SLIM_RESULTS_FILE" <<'COMPARE_SCRIPT'
import json
import sys

results_file = sys.argv[1]
slim_results_file = sys.argv[2]

try:
    with open(results_file, "r") as f:
        original = json.load(f)
    with open(slim_results_file, "r") as f:
        slim = json.load(f)

    original_passed = sum(1 for t in original.get("tests", []) if t.get("status") == "passed")
    original_failed = sum(1 for t in original.get("tests", []) if t.get("status") == "failed")
    slim_passed = sum(1 for t in slim.get("tests", []) if t.get("status") == "passed")
    slim_failed = sum(1 for t in slim.get("tests", []) if t.get("status") == "failed")

    print(f"Original: {original_passed} passed, {original_failed} failed")
    print(f"Slim:     {slim_passed} passed, {slim_failed} failed")

    if original_passed == slim_passed:
        print("SUCCESS: Both images passed the same number of tests!")
        sys.exit(0)
    else:
        print("WARNING: Different number of tests passed")
        sys.exit(1)
except Exception as e:
    print(f"Error comparing results: {e}")
    sys.exit(1)
COMPARE_SCRIPT

COMPARE_EXIT=$?

echo ""
echo "============================================="
echo "Test Complete"
echo "============================================="

exit $COMPARE_EXIT

