#!/bin/bash

# final_comprehensive_test.sh - Sequential testing of ALL hvsum functionality
# This script runs commands one by one and captures everything

TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
OUTPUT_FILE="final_test_results_${TIMESTAMP}.txt"

echo "=== COMPREHENSIVE HVSUM TEST SUITE ===" > "$OUTPUT_FILE"
echo "Started: $(date)" >> "$OUTPUT_FILE"
echo "Testing ALL functionality, flags, accuracy, and edge cases" >> "$OUTPUT_FILE"
echo "=========================================" >> "$OUTPUT_FILE"

# Test counter
TEST_NUM=1

run_test() {
    echo "" >> "$OUTPUT_FILE"
    echo "TEST $TEST_NUM: $1" >> "$OUTPUT_FILE"
    echo "Command: $2" >> "$OUTPUT_FILE"
    echo "---" >> "$OUTPUT_FILE"
    eval "$2" >> "$OUTPUT_FILE" 2>&1
    echo "Exit code: $?" >> "$OUTPUT_FILE"
    echo "=========================================" >> "$OUTPUT_FILE"
    ((TEST_NUM++))
}

# Kill any hanging processes
pkill -f hvsum 2>/dev/null || true

echo "Starting comprehensive hvsum tests..."

# BASIC FUNCTIONALITY TESTS
run_test "Version Check" "./hvsum --version"
run_test "Help Display" "./hvsum --help"
run_test "No Arguments (Should show usage)" "./hvsum"

# URL SUMMARIZATION TESTS - Different Lengths
run_test "Short Summary" "./hvsum -l short --no-qna https://example.com"
run_test "Medium Summary" "./hvsum -l medium --no-qna https://httpbin.org/html"
run_test "Long Summary" "./hvsum -l long --no-qna https://httpbin.org/json"
run_test "Detailed Summary" "./hvsum -l detailed --no-qna https://httpbin.org/robots.txt"

# SEARCH FUNCTIONALITY TESTS
run_test "Search Only - Simple Query" "./hvsum -s --no-qna 'Python programming'"
run_test "Search Only - Technical Query" "./hvsum -s --no-qna 'machine learning algorithms'"
run_test "Search Only - Recent Topic" "./hvsum -s --no-qna 'artificial intelligence 2024'"

# URL + SEARCH ENHANCEMENT TESTS
run_test "URL with Search Enhancement" "./hvsum -s --no-qna https://httpbin.org/html"

# OUTPUT FORMAT TESTS
run_test "Markdown Output" "./hvsum -m --no-qna https://httpbin.org/json"
run_test "Markdown + Search" "./hvsum -m -s --no-qna 'Docker containers'"
run_test "Outline Generation" "./hvsum -o --no-qna https://httpbin.org/html"
run_test "Markdown + Outline" "./hvsum -m -o --no-qna https://httpbin.org/json"

# FILE OPERATIONS TESTS
TEST_DIR=$(mktemp -d)
run_test "Save to Text File" "./hvsum --no-qna -w '${TEST_DIR}/test.txt' https://httpbin.org/html"
run_test "Save to Markdown File" "./hvsum -m --no-qna -w '${TEST_DIR}/test.md' https://httpbin.org/json"
run_test "Check if files were created" "ls -la ${TEST_DIR}/"

# CACHE TESTS
run_test "Clean Cache" "./hvsum --clean-cache"
run_test "No Cache Flag" "./hvsum --no-cache --no-qna https://httpbin.org/html"

# COMPLEX FLAG COMBINATIONS
run_test "All Flags Combined" "./hvsum -m -s -l long -o --no-cache --no-qna 'JavaScript frameworks'"
run_test "Save + Markdown + Search" "./hvsum -m -s --no-qna -w '${TEST_DIR}/combo.md' 'React vs Vue'"

# SESSION MANAGEMENT TESTS
run_test "List Sessions" "./hvsum --list-sessions"

# ERROR HANDLING TESTS
run_test "Invalid URL" "./hvsum --no-qna https://this-definitely-does-not-exist-12345.invalid"
run_test "Invalid Length Parameter" "./hvsum -l invalid --no-qna https://httpbin.org/html"

# INTERACTIVE Q&A ACCURACY TESTS
echo "Starting Interactive Q&A Tests..." >> "$OUTPUT_FILE"

# Test 1: Basic Q&A
run_test "Basic Interactive Q&A" "echo -e 'What is this page about?\nWhat are the main features?\n/exit\nn\n' | ./hvsum https://httpbin.org/html"

# Test 2: Search-enhanced Q&A
run_test "Search-Enhanced Q&A" "echo -e 'What are the latest developments in this field?\nWhat are the practical applications?\n/exit\nn\n' | ./hvsum -s https://httpbin.org/html"

# Test 3: Follow-up questions that should trigger search
run_test "Search Trigger Test" "echo -e 'What is machine learning?\nWhat are the current market trends for ML in 2024?\nWho are the major companies investing in this?\n/exit\nn\n' | ./hvsum -s https://en.wikipedia.org/wiki/Machine_learning"

# Test 4: Context-based follow-up questions
run_test "Context Follow-up Test" "echo -e 'What are the main components?\nHow do these components interact?\nWhat happens if one fails?\n/exit\nn\n' | ./hvsum https://en.wikipedia.org/wiki/Microservices"

# Test 5: Academic accuracy test
run_test "Academic Content Accuracy" "echo -e 'What are the key research findings?\nWhat methodology was used?\nWhat are the limitations of this research?\n/exit\nn\n' | ./hvsum -s https://en.wikipedia.org/wiki/CRISPR"

# Test 6: Controversial topic handling
run_test "Controversial Topic Handling" "echo -e 'What are the main arguments for and against?\nWhat does recent research say about this?\nAre there any unbiased studies?\n/exit\nn\n' | ./hvsum -s https://en.wikipedia.org/wiki/Climate_change"

# Test 7: Technical deep-dive questions
run_test "Technical Deep Dive" "echo -e 'How does this technology work internally?\nWhat are the performance implications?\nWhat are the security considerations?\n/exit\nn\n' | ./hvsum -s https://en.wikipedia.org/wiki/Blockchain"

# Test 8: Historical accuracy test
run_test "Historical Accuracy Test" "echo -e 'What were the main causes?\nWhat were the consequences?\nHow do modern historians view this?\n/exit\nn\n' | ./hvsum -s https://en.wikipedia.org/wiki/World_War_II"

# HALLUCINATION AND ACCURACY TESTS
run_test "Factual Accuracy - Science" "echo -e 'What is the speed of light in vacuum?\nWhat is the chemical formula for water?\nWhat is the largest planet in our solar system?\n/exit\nn\n' | ./hvsum -s https://en.wikipedia.org/wiki/Physics"

run_test "Source Citation Test" "echo -e 'Can you provide specific sources for these claims?\nWhat evidence supports this information?\nAre there any conflicting viewpoints?\n/exit\nn\n' | ./hvsum -s https://en.wikipedia.org/wiki/Artificial_intelligence"

# SESSION PERSISTENCE TESTS
run_test "Session Save Test" "echo -e 'What is quantum computing?\n/exit\ns\ntest_session_${TIMESTAMP}\n' | ./hvsum -s https://en.wikipedia.org/wiki/Quantum_computing"

run_test "Session Resume Test" "echo -e 'Tell me more about quantum algorithms\n/exit\nn\n' | ./hvsum --session test_session_${TIMESTAMP}"

# PERFORMANCE TESTS
run_test "Large Content Test" "./hvsum --no-qna https://en.wikipedia.org/wiki/List_of_countries_by_population"

run_test "Multiple Quick Tests" "./hvsum --no-qna https://httpbin.org/html && ./hvsum --no-qna https://httpbin.org/json && ./hvsum --no-qna https://httpbin.org/robots.txt"

# EDGE CASES
run_test "Empty Query Search" "./hvsum -s --no-qna ''"
run_test "Very Long Query" "./hvsum -s --no-qna 'this is a very long query that goes on and on and tests how the system handles extremely verbose input that might cause issues with processing or display formatting'"

# COMMAND HELP TESTS
run_test "Interactive Help Commands" "echo -e '/help\n/history\n/exit\nn\n' | ./hvsum https://httpbin.org/html"

# Clean up
rm -rf "$TEST_DIR"

echo "" >> "$OUTPUT_FILE"
echo "=== TEST SUITE COMPLETED ===" >> "$OUTPUT_FILE"
echo "Finished: $(date)" >> "$OUTPUT_FILE"
echo "Results saved to: $OUTPUT_FILE" >> "$OUTPUT_FILE"

echo "Comprehensive test completed. Results in: $OUTPUT_FILE"
echo "Total tests run: $((TEST_NUM - 1))" 