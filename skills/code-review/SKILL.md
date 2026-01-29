---
name: Code Review
description: Review code for bugs, security issues, and best practices
version: 1.0.0
metadata:
  emoji: "🔍"
  tags:
    - code-quality
    - security
    - review
---

## Instructions

When the user asks you to review code, follow these guidelines:

### Review Checklist

1. **Correctness**
   - Does the code do what it's supposed to do?
   - Are there any logic errors?
   - Are edge cases handled?

2. **Security**
   - Input validation and sanitization
   - SQL injection vulnerabilities
   - XSS vulnerabilities
   - Command injection
   - Path traversal
   - Sensitive data exposure
   - Authentication/authorization issues

3. **Code Quality**
   - Is the code readable and maintainable?
   - Are variable/function names descriptive?
   - Is there unnecessary complexity?
   - Are there code duplications?

4. **Performance**
   - Are there obvious performance issues?
   - N+1 queries
   - Unnecessary loops or computations
   - Memory leaks

5. **Error Handling**
   - Are errors properly caught and handled?
   - Are error messages helpful?
   - Is there proper logging?

6. **Testing**
   - Are there tests for the new code?
   - Do the tests cover edge cases?

### Review Format

Structure your review as follows:

```
## Summary
[Brief overview of what the code does and overall assessment]

## Critical Issues
[Must-fix issues: bugs, security vulnerabilities]

## Suggestions
[Improvements and best practices recommendations]

## Questions
[Clarifications needed about intent or design decisions]

## Positive Aspects
[Good practices observed in the code]
```

### Guidelines

- Be constructive, not critical
- Explain the "why" behind suggestions
- Provide concrete examples for improvements
- Prioritize issues by severity
- Acknowledge good practices
