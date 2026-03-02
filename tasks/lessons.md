# Lessons

## 2026-03-02 Installer Asset Location Must Be Package-Safe
- Incident/bug: Installers assumed repo-relative `agents/` paths, breaking packaged installs.
- Root cause pattern: Runtime path resolution relied on monorepo layout instead of shipped assets.
- Early signal missed: Packaging manifests claimed assets but package-local directories were absent.
- Preventative rule: Any installer that reads templates/content must default to packaged resources and treat repo-root access as an explicit dev override.
- Validation added (test/check/alert): npm pack smoke check plus temp-dir runtime install smoke checks.
- Next trigger to detect sooner: During PR review, verify `npm pack`/wheel/go install outputs contain all runtime assets.
