# Security Audit

You are tasked with performing a security audit on the codebase.

## Instructions
1. Use the audit-security skill to scan for vulnerabilities.
2. Run security scanning tools appropriate for the project language (e.g., govulncheck for Go, npm audit for Node.js).
3. Check for:
   - Hardcoded credentials or secrets
   - Known vulnerable dependencies
   - Unsafe code patterns (e.g., command injection, SQL injection)
   - Insecure file permissions
4. Report all findings clearly.

## Expected Outcome
- AUDIT: PASSED — No security issues found.
- AUDIT: FAILED — Security issues found. Create remediation tasks.
