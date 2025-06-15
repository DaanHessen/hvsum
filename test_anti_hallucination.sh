#!/bin/bash

# Test Anti-Hallucination Improvements
# This script tests scenarios that previously showed hallucinations

echo "=== ANTI-HALLUCINATION TEST SUITE ==="
echo "Testing improved hvsum with fact verification..."
echo

# Test 1: Simple content that should not be embellished
echo "Test 1: Simple factual content (should not add drama or speculation)"
echo "Content: Basic product information"
echo

echo "Test data: ProductX costs $50 and weighs 2 pounds." | ./hvsum --length medium 2>/dev/null

echo
echo "----------------------------------------"
echo

# Test 2: Minimal content test
echo "Test 2: Minimal content (should not fabricate details)"
echo "Content: Very basic information"
echo

echo "Test data: Company ABC was founded in 2020." | ./hvsum --length short 2>/dev/null

echo
echo "----------------------------------------"
echo

# Test 3: Test with URL to see if it stays grounded
echo "Test 3: URL-based summarization (checking for source grounding)"
echo "Testing with a simple webpage..."
echo

# Test with a simple URL (if available) or create mock test
if command -v curl &> /dev/null; then
    echo "Simple test content from: https://httpbin.org/json" | ./hvsum --length short 2>/dev/null
else
    echo "curl not available, skipping URL test"
fi

echo
echo "----------------------------------------"
echo

echo "Test completed. Review outputs for:"
echo "1. No dramatic language or storytelling"
echo "2. No invented details beyond the source"
echo "3. Factual, neutral tone"
echo "4. Source-grounded information only" 