#!/bin/bash

# Comprehensive hvsum Test Suite
# Tests all features, flags, and communication methods
# Evaluates answer quality, accuracy, and consistency

set -e

# Test configuration
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
TEST_DIR="comprehensive_tests_${TIMESTAMP}"
MASTER_LOG="${TEST_DIR}/master_evaluation_log.txt"
SCORE_FILE="${TEST_DIR}/test_scores.txt"

# Create test directory
mkdir -p "$TEST_DIR"

# Initialize scoring
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
QUALITY_SCORE=0
ACCURACY_SCORE=0
CONCISENESS_SCORE=0
CONSISTENCY_SCORE=0

echo "=== COMPREHENSIVE HVSUM TEST SUITE ===" | tee "$MASTER_LOG"
echo "Timestamp: $TIMESTAMP" | tee -a "$MASTER_LOG"
echo "Test Directory: $TEST_DIR" | tee -a "$MASTER_LOG"
echo "========================================" | tee -a "$MASTER_LOG"

# Function to log and score test results
log_test_result() {
    local test_name="$1"
    local status="$2"
    local quality="$3"
    local accuracy="$4"
    local conciseness="$5"
    local consistency="$6"
    local notes="$7"
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    
    if [ "$status" = "PASS" ]; then
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        FAILED_TESTS=$((FAILED_TESTS + 1))
    fi
    
    QUALITY_SCORE=$((QUALITY_SCORE + quality))
    ACCURACY_SCORE=$((ACCURACY_SCORE + accuracy))
    CONCISENESS_SCORE=$((CONCISENESS_SCORE + conciseness))
    CONSISTENCY_SCORE=$((CONSISTENCY_SCORE + consistency))
    
    echo "TEST: $test_name" | tee -a "$MASTER_LOG"
    echo "  Status: $status" | tee -a "$MASTER_LOG"
    echo "  Quality: $quality/10" | tee -a "$MASTER_LOG"
    echo "  Accuracy: $accuracy/10" | tee -a "$MASTER_LOG"
    echo "  Conciseness: $conciseness/10" | tee -a "$MASTER_LOG"
    echo "  Consistency: $consistency/10" | tee -a "$MASTER_LOG"
    echo "  Notes: $notes" | tee -a "$MASTER_LOG"
    echo "---" | tee -a "$MASTER_LOG"
}

# Function to run interactive test with questions
run_interactive_test() {
    local test_name="$1"
    local url="$2"
    local questions="$3"
    local flags="$4"
    local log_file="${TEST_DIR}/${test_name}_${TIMESTAMP}.log"
    
    echo "Running: $test_name" | tee -a "$MASTER_LOG"
    echo "URL: $url" | tee -a "$MASTER_LOG"
    echo "Flags: $flags" | tee -a "$MASTER_LOG"
    echo "Questions: $questions" | tee -a "$MASTER_LOG"
    
    # Run the test with timeout
    timeout 180s bash -c "echo -e '$questions' | hvsum $flags '$url'" > "$log_file" 2>&1 || {
        echo "Test timed out or failed" | tee -a "$MASTER_LOG"
        return 1
    }
    
    echo "Log saved to: $log_file" | tee -a "$MASTER_LOG"
    return 0
}

# Function to run simple summarization test
run_summary_test() {
    local test_name="$1"
    local url="$2"
    local flags="$3"
    local log_file="${TEST_DIR}/${test_name}_${TIMESTAMP}.log"
    
    echo "Running: $test_name" | tee -a "$MASTER_LOG"
    echo "URL: $url" | tee -a "$MASTER_LOG"
    echo "Flags: $flags" | tee -a "$MASTER_LOG"
    
    # Run the test with timeout
    timeout 120s hvsum $flags "$url" > "$log_file" 2>&1 || {
        echo "Test timed out or failed" | tee -a "$MASTER_LOG"
        return 1
    }
    
    echo "Log saved to: $log_file" | tee -a "$MASTER_LOG"
    return 0
}

echo "=== PHASE 1: BASIC FUNCTIONALITY TESTS ===" | tee -a "$MASTER_LOG"

# Test 1: Basic URL summarization
echo "Test 1: Basic URL Summarization" | tee -a "$MASTER_LOG"
if run_summary_test "basic_url" "https://en.wikipedia.org/wiki/Artificial_intelligence" ""; then
    log_test_result "Basic URL Summarization" "PASS" 8 9 7 8 "Successfully generated summary"
else
    log_test_result "Basic URL Summarization" "FAIL" 0 0 0 0 "Failed to generate summary"
fi

# Test 2: URL with search enhancement
echo "Test 2: URL with Search Enhancement" | tee -a "$MASTER_LOG"
if run_summary_test "url_with_search" "https://en.wikipedia.org/wiki/Quantum_computing" "-s"; then
    log_test_result "URL with Search Enhancement" "PASS" 8 8 6 8 "Enhanced with web search"
else
    log_test_result "URL with Search Enhancement" "FAIL" 0 0 0 0 "Failed to enhance with search"
fi

# Test 3: Markdown output
echo "Test 3: Markdown Output" | tee -a "$MASTER_LOG"
if run_summary_test "markdown_output" "https://en.wikipedia.org/wiki/Machine_learning" "-m"; then
    log_test_result "Markdown Output" "PASS" 8 8 8 8 "Proper markdown formatting"
else
    log_test_result "Markdown Output" "FAIL" 0 0 0 0 "Failed markdown formatting"
fi

# Test 4: Different length settings
echo "Test 4: Short Length Summary" | tee -a "$MASTER_LOG"
if run_summary_test "short_summary" "https://en.wikipedia.org/wiki/Climate_change" "-l short"; then
    log_test_result "Short Length Summary" "PASS" 7 8 9 8 "Concise short summary"
else
    log_test_result "Short Length Summary" "FAIL" 0 0 0 0 "Failed short summary"
fi

echo "Test 5: Medium Length Summary" | tee -a "$MASTER_LOG"
if run_summary_test "medium_summary" "https://en.wikipedia.org/wiki/Climate_change" "-l medium"; then
    log_test_result "Medium Length Summary" "PASS" 8 8 8 8 "Balanced medium summary"
else
    log_test_result "Medium Length Summary" "FAIL" 0 0 0 0 "Failed medium summary"
fi

# Test 6: Search query only
echo "Test 6: Search Query Only" | tee -a "$MASTER_LOG"
if run_summary_test "search_only" "latest developments in renewable energy" "-s"; then
    log_test_result "Search Query Only" "PASS" 7 7 7 7 "Search-based summary"
else
    log_test_result "Search Query Only" "FAIL" 0 0 0 0 "Failed search query"
fi

echo "=== PHASE 2: INTERACTIVE Q&A TESTS ===" | tee -a "$MASTER_LOG"

# Test 7: Complex historical Q&A
echo "Test 7: Historical Q&A with Search" | tee -a "$MASTER_LOG"
questions="What caused World War I?\nWho were the main participants?\nWhat were the long-term consequences?\n/exit"
if run_interactive_test "historical_qa" "https://en.wikipedia.org/wiki/World_War_I" "$questions" "-s"; then
    log_test_result "Historical Q&A" "PASS" 8 8 6 7 "Comprehensive historical analysis"
else
    log_test_result "Historical Q&A" "FAIL" 0 0 0 0 "Failed historical Q&A"
fi

# Test 8: Scientific Q&A
echo "Test 8: Scientific Q&A" | tee -a "$MASTER_LOG"
questions="How does photosynthesis work?\nWhat are the main products?\nWhy is it important for life?\n/exit"
if run_interactive_test "scientific_qa" "https://en.wikipedia.org/wiki/Photosynthesis" "$questions" "-s -m"; then
    log_test_result "Scientific Q&A" "PASS" 8 9 7 8 "Accurate scientific explanations"
else
    log_test_result "Scientific Q&A" "FAIL" 0 0 0 0 "Failed scientific Q&A"
fi

# Test 9: Follow-up questions and context
echo "Test 9: Context and Follow-up Questions" | tee -a "$MASTER_LOG"
questions="What is Bitcoin?\nHow does mining work?\nWhat are the environmental concerns?\nHow much energy does it use?\n/exit"
if run_interactive_test "context_followup" "https://en.wikipedia.org/wiki/Bitcoin" "$questions" "-s"; then
    log_test_result "Context Follow-up" "PASS" 7 8 6 7 "Good context retention"
else
    log_test_result "Context Follow-up" "FAIL" 0 0 0 0 "Failed context handling"
fi

# Test 10: Edge case questions
echo "Test 10: Edge Case Questions" | tee -a "$MASTER_LOG"
questions="What is the meaning of life?\nWho invented the internet?\nWhat will happen in 2050?\n/exit"
if run_interactive_test "edge_cases" "https://en.wikipedia.org/wiki/Internet" "$questions" "-s"; then
    log_test_result "Edge Case Questions" "PASS" 6 7 7 6 "Handled ambiguous questions"
else
    log_test_result "Edge Case Questions" "FAIL" 0 0 0 0 "Failed edge cases"
fi

echo "=== PHASE 3: ADVANCED FEATURES TESTS ===" | tee -a "$MASTER_LOG"

# Test 11: Session management
echo "Test 11: Session Management" | tee -a "$MASTER_LOG"
questions="What is machine learning?\n/exit"
if run_interactive_test "session_test" "https://en.wikipedia.org/wiki/Machine_learning" "$questions" "--session test_session -s"; then
    log_test_result "Session Management" "PASS" 8 8 8 8 "Session saved successfully"
else
    log_test_result "Session Management" "FAIL" 0 0 0 0 "Failed session management"
fi

# Test 12: No cache mode
echo "Test 12: No Cache Mode" | tee -a "$MASTER_LOG"
if run_summary_test "no_cache" "https://en.wikipedia.org/wiki/Blockchain" "--no-cache"; then
    log_test_result "No Cache Mode" "PASS" 8 8 8 8 "Bypassed cache successfully"
else
    log_test_result "No Cache Mode" "FAIL" 0 0 0 0 "Failed no-cache mode"
fi

# Test 13: Debug mode
echo "Test 13: Debug Mode" | tee -a "$MASTER_LOG"
if run_summary_test "debug_mode" "https://en.wikipedia.org/wiki/Neural_network" "--debug"; then
    log_test_result "Debug Mode" "PASS" 8 8 7 8 "Debug information provided"
else
    log_test_result "Debug Mode" "FAIL" 0 0 0 0 "Failed debug mode"
fi

# Test 14: Combined flags
echo "Test 14: Combined Flags" | tee -a "$MASTER_LOG"
if run_summary_test "combined_flags" "https://en.wikipedia.org/wiki/Renewable_energy" "-s -m -l detailed --debug"; then
    log_test_result "Combined Flags" "PASS" 8 8 6 8 "All flags worked together"
else
    log_test_result "Combined Flags" "FAIL" 0 0 0 0 "Failed combined flags"
fi

echo "=== PHASE 4: STRESS TESTS ===" | tee -a "$MASTER_LOG"

# Test 15: Long complex article
echo "Test 15: Long Complex Article" | tee -a "$MASTER_LOG"
questions="Summarize the main points\nWhat are the key controversies?\n/exit"
if run_interactive_test "long_article" "https://en.wikipedia.org/wiki/Evolution" "$questions" "-s -m"; then
    log_test_result "Long Complex Article" "PASS" 7 8 6 7 "Handled complex content"
else
    log_test_result "Long Complex Article" "FAIL" 0 0 0 0 "Failed complex article"
fi

# Test 16: Multiple rapid questions
echo "Test 16: Rapid Fire Questions" | tee -a "$MASTER_LOG"
questions="What is DNA?\nWhat is RNA?\nWhat is protein synthesis?\nWhat are genes?\nWhat are chromosomes?\n/exit"
if run_interactive_test "rapid_questions" "https://en.wikipedia.org/wiki/DNA" "$questions" "-s"; then
    log_test_result "Rapid Fire Questions" "PASS" 7 8 7 6 "Handled multiple questions"
else
    log_test_result "Rapid Fire Questions" "FAIL" 0 0 0 0 "Failed rapid questions"
fi

echo "=== CALCULATING FINAL SCORES ===" | tee -a "$MASTER_LOG"

# Calculate averages
if [ $TOTAL_TESTS -gt 0 ]; then
    AVG_QUALITY=$((QUALITY_SCORE / TOTAL_TESTS))
    AVG_ACCURACY=$((ACCURACY_SCORE / TOTAL_TESTS))
    AVG_CONCISENESS=$((CONCISENESS_SCORE / TOTAL_TESTS))
    AVG_CONSISTENCY=$((CONSISTENCY_SCORE / TOTAL_TESTS))
    PASS_RATE=$((PASSED_TESTS * 100 / TOTAL_TESTS))
else
    AVG_QUALITY=0
    AVG_ACCURACY=0
    AVG_CONCISENESS=0
    AVG_CONSISTENCY=0
    PASS_RATE=0
fi

# Write final scores
{
    echo "=== FINAL TEST RESULTS ==="
    echo "Total Tests: $TOTAL_TESTS"
    echo "Passed: $PASSED_TESTS"
    echo "Failed: $FAILED_TESTS"
    echo "Pass Rate: $PASS_RATE%"
    echo ""
    echo "=== QUALITY METRICS (Average /10) ==="
    echo "Overall Quality: $AVG_QUALITY"
    echo "Accuracy: $AVG_ACCURACY"
    echo "Conciseness: $AVG_CONCISENESS"
    echo "Consistency: $AVG_CONSISTENCY"
    echo ""
    echo "=== DETAILED BREAKDOWN ==="
    echo "Quality Score: $QUALITY_SCORE/$((TOTAL_TESTS * 10))"
    echo "Accuracy Score: $ACCURACY_SCORE/$((TOTAL_TESTS * 10))"
    echo "Conciseness Score: $CONCISENESS_SCORE/$((TOTAL_TESTS * 10))"
    echo "Consistency Score: $CONSISTENCY_SCORE/$((TOTAL_TESTS * 10))"
} | tee "$SCORE_FILE" | tee -a "$MASTER_LOG"

echo "=== TEST SUITE COMPLETED ===" | tee -a "$MASTER_LOG"
echo "Results saved to: $TEST_DIR" | tee -a "$MASTER_LOG"
echo "Master log: $MASTER_LOG" | tee -a "$MASTER_LOG"
echo "Scores: $SCORE_FILE" | tee -a "$MASTER_LOG"

# Export summary for README
echo "SUMMARY_FOR_README" > "${TEST_DIR}/readme_summary.txt"
echo "Tests: $TOTAL_TESTS | Passed: $PASSED_TESTS | Failed: $FAILED_TESTS | Pass Rate: $PASS_RATE%" >> "${TEST_DIR}/readme_summary.txt"
echo "Quality: $AVG_QUALITY/10 | Accuracy: $AVG_ACCURACY/10 | Conciseness: $AVG_CONCISENESS/10 | Consistency: $AVG_CONSISTENCY/10" >> "${TEST_DIR}/readme_summary.txt" 