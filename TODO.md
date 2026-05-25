- [x] Update Units API + CRD schemas: add status.lastSuccessfulCommitSHA
- [x] Update controller diff baseline to use lastSuccessfulCommitSHA (initially empty)

- [x] Implement runner Job status check; update lastSuccessfulCommitSHA only when the corresponding runner Job reports completion successfully


- [x] Ensure initialization behavior: lastSuccessfulCommitSHA empty should cause runner/git logic to derive head-1 (implemented in controller.go)
- [x] Update unit tests to cover success/failure and baseline selection
- [x] Run go test ./... and fix any failing tests
