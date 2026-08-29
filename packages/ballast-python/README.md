# ballast-python

Python CLI package for installing Ballast agent rules and skills.

## Install

```bash
uv tool install ballast-python
```

## Usage

```bash
ballast-python install --target cursor --all
uvx --from ballast-python ballast-python install --target codex --agent linting
ballast-python install --target codex --skill speckit-bootstrap
ballast-python install --target cursor --agent linting --patch
```
