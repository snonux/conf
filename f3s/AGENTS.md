Follow file:///home/paul/Notes/snippets/f3s/f3s.md
<<<<<<< HEAD
||||||| 7b4d629
=======

## Security Policy

- **Never commit secrets to git.** This includes SSH private keys, API tokens, passwords, and any other sensitive credentials.
- Secrets must be deployed as Kubernetes Secrets directly via `kubectl create secret` or through a secrets management solution.
- If a secret is accidentally committed, it must be rotated immediately and pruned from git history using `git-filter-repo`.
>>>>>>> 529caf525d3c8594bcf0208697629827113dc1fc
