---
project: Cortex
component: Unknown
phase: Build
date_created: 2026-01-16T21:14:56
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:18:06.802606
---

# Contributing to Cortex Evaluator

First off, thank you for considering contributing to Cortex Evaluator! It's people like you that make it a great tool for the developer community.

## 🌈 How Can I Contribute?

### Reporting Bugs
- Use the GitHub issue tracker.
- Describe the bug and include steps to reproduce it.
- Include information about your environment (OS, Node version, Python version).

### Suggesting Enhancements
- Open a GitHub issue with the "enhancement" label.
- Explain the "why" behind the feature.

### Pull Requests
1. Fork the repo and create your branch from `main`.
2. If you've added code that should be tested, add tests.
3. If you've changed APIs, update the documentation.
4. Ensure the test suite passes.
5. Make sure your code lints.
6. Issue that pull request!

## 💻 Development Workflow

1. **Setup**: Follow the instructions in the [README](README.md) to set up your development environment.
2. **Branching**: Use descriptive branch names: `feat/add-new-provider`, `fix/broken-canvas-nodes`.
3. **Commit Messages**: We follow [Conventional Commits](https://www.conventionalcommits.org/):
   - `feat: ...` for new features
   - `fix: ...` for bug fixes
   - `docs: ...` for documentation changes
   - `refactor: ...` for code changes that neither fix a bug nor add a feature

## 🎨 Code Style

### Frontend
- Use TypeScript for all new code.
- Follow the React 19 functional component patterns.
- Use Tailwind CSS for styling.
- Ensure components are responsive and support dark mode.

### Backend
- Follow PEP 8 for Python code.
- Use type hints for all function signatures.
- Document complex logic with docstrings.
- Ensure all new endpoints are included in the auto-generated Swagger docs.

## 📜 Pull Request Template

When opening a PR, please use the following template in the description:

```markdown
## Summary
[Brief description of changes]

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Documentation update
- [ ] Refactor

## How Has This Been Tested?
[Describe your testing process]

## Checklist
- [ ] My code follows the style guidelines of this project
- [ ] I have performed a self-review of my own code
- [ ] I have commented my code, particularly in hard-to-understand areas
- [ ] I have made corresponding changes to the documentation
- [ ] My changes generate no new warnings
```

## ⚖️ License
By contributing, you agree that your contributions will be licensed under its MIT License.
