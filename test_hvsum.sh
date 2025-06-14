#!/bin/bash

# test_hvsum.sh
# A script to automate testing of the hvsum interactive Q&A functionality.

# Ensure hvsum is in the path or in the current directory
if ! command -v ./hvsum &> /dev/null; then
    echo "Error: ./hvsum not found. Please build the binary and run this script from the project root."
    exit 1
fi

# Create a directory for test logs
LOG_DIR="test_logs"
mkdir -p "$LOG_DIR"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
MASTER_LOG_FILE="${LOG_DIR}/master_log_${TIMESTAMP}.txt"

# --- Test Cases ---
# Each test case is a function that defines a URL and a series of questions.
# The questions are designed to test various aspects of the Q&A system.

test_case_1_WW2() {
    local test_name="WW2_Hitler"
    local url="https://en.wikipedia.org/wiki/World_War_II"
    local questions=(
        "How did Hitler die?"
        "Can you give me more detail on his death, in a couple of paragraphs?"
        "Did Mein Kampf state the reason for his suicide?"
        "What about his wife? Was she influenced by Hitler to end her life, or was it her own voluntary decision? Search for specifics on this."
        "What do you mean by cyanide poisoning? Explain the mechanism."
        "How did the gas chambers work? What was Zyklon B?"
        "What fueled Hitler's hate towards Jews? Wasn't he a Jew himself? Check his family history."
    )
    run_test "$test_name" "$url" "${questions[@]}"
}

test_case_2_Apollo() {
    local test_name="Apollo_11"
    local url="https://en.wikipedia.org/wiki/Apollo_11"
    local questions=(
        "What was the primary objective of the Apollo 11 mission?"
        "Who were the astronauts on board?"
        "What were the 'first words' spoken on the moon?"
        "Were there any major malfunctions during the landing? I'm looking for details about computer alarms."
        "What was the '1202 alarm' specifically? Perform a search if the context is insufficient."
        "How much did the Apollo program cost in today's dollars?"
        "What happened to the command module after the mission?"
    )
    run_test "$test_name" "$url" "${questions[@]}"
}

test_case_3_CRISPR() {
    local test_name="CRISPR"
    local url="https://en.wikipedia.org/wiki/CRISPR"
    local questions=(
        "What does CRISPR stand for?"
        "Explain in simple terms how CRISPR-Cas9 works for gene editing."
        "Who won the Nobel Prize for this discovery? And in what year?"
        "Are there ethical concerns surrounding CRISPR technology? Summarize the main points."
        "What is a 'gene drive' and how does it relate to CRISPR? Search for this specifically."
        "Has CRISPR been used in human trials? What were the results?"
        "What are some of the potential future applications of this technology?"
    )
    run_test "$test_name" "$url" "${questions[@]}"
}

# --- Test Runner ---
# This function takes the test case parameters and executes the test.

run_test() {
    local test_name="$1"
    local url="$2"
    shift 2
    local questions=("$@")

    local test_log_file="${LOG_DIR}/${test_name}_${TIMESTAMP}.log"
    
    echo "--- Starting Test Case: $test_name ---" | tee -a "$MASTER_LOG_FILE"
    echo "URL: $url" | tee -a "$MASTER_LOG_FILE"
    echo "Log file: $test_log_file" | tee -a "$MASTER_LOG_FILE"
    echo "" | tee -a "$MASTER_LOG_FILE"

    # Prepare the questions to be piped into the hvsum process
    local input_commands=""
    for q in "${questions[@]}"; do
        input_commands+="${q}\n"
    done
    # Add exit command at the end and save the session
    input_commands+="/exit\ns\n${test_name}\n"

    # Run hvsum with the questions piped in.
    # Flags: -m (markdown), -s (search), --no-cache
    # The timeout is set to 5 minutes (300s) to allow for network and AI latency.
    {
        echo "Running test: $test_name"
        printf "%b" "$input_commands" | timeout 300s ./hvsum -m -s --no-cache "$url"
        echo -e "\n--- Test Case Finished: $test_name ---\n\n"
    } &> "$test_log_file"

    # Check if the test completed successfully or timed out
    if [ $? -eq 124 ]; then
        echo "TEST CASE TIMED OUT: $test_name" | tee -a "$MASTER_LOG_FILE"
        echo "TEST CASE TIMED OUT: $test_name" >> "$test_log_file"
    else
        echo "Test Case Completed: $test_name" | tee -a "$MASTER_LOG_FILE"
    fi
    
    # Append individual log to master log for a complete record
    cat "$test_log_file" >> "$MASTER_LOG_FILE"
    echo -e "\n\n" >> "$MASTER_LOG_FILE"
}

# --- Main Execution ---

echo "Starting hvsum test suite..." | tee -a "$MASTER_LOG_FILE"
echo "Timestamp: $TIMESTAMP" | tee -a "$MASTER_LOG_FILE"
echo "=================================" | tee -a "$MASTER_LOG_FILE"
echo "" | tee -a "$MASTER_LOG_FILE"

# Run all test cases
test_case_1_WW2
test_case_2_Apollo
test_case_3_CRISPR

echo "=================================" | tee -a "$MASTER_LOG_FILE"
echo "Test suite finished." | tee -a "$MASTER_LOG_FILE"
echo "Master log file located at: $MASTER_LOG_FILE"

# Provide a summary of the results
echo -e "\n--- Test Summary ---"
grep -E "--- Starting Test Case:|Test Case Completed:|TEST CASE TIMED OUT:" "$MASTER_LOG_FILE"
echo "--------------------"

exit 0 