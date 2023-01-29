---
title: "How to Run Hatch Within Github Workflow"
date: 2023-01-28T22:10:02+08:00
draft: false
tags: ["Hatch", "Python"]
---

Hatch support support define matrix within a environment ([matrix docs](https://hatch.pypa.io/latest/config/environment/advanced/#matrix)), this is so useful when you want to run test against different python version. For example:

```toml
[tool.hatch.envs.test]
dependencies = [
  "pytest",
]

[[tool.hatch.envs.test.matrix]]
python = ["3.7", "3.8", "3.9", "3.10", "3.11"]

[tool.hatch.envs.test.scripts]
test = "pytest tests/"
```

But how to run hatch within github workflow? This post will show you how to do it.

In github workflow you can also define matrix to run job against different python version,
maybe you want set CI like this:

```yaml
name: "CI"

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  run:
    name: "tests & coverage"
    runs-on: ubuntu-latest
    strategy:
      matrix:
        python-version: ["3.7", "3.8", "3.9", "3.10", "3.11"]

    steps:
      - uses: actions/checkout@v3
      - name: Set up Python ${{ matrix.python-version }}
        uses: actions/setup-python@v4
        with:
          python-version: ${{ matrix.python-version }}

      - name: Install hatch
        run: python -m pip install hatch

      # then run test
```
If you directly run `hatch run test:test`, Hatch will try to set virtualenv for each python version.It will failed, because only one system python is installed at each strategy.

Fortunately, hatch can use `+py` to specify the python version to run env command, so you can change the CI config like this:

```yaml
- name: Tests
  run: hatch run +py=${{ matrix.python-version }}  test:test
```

The full CI config:

```yaml
name: "CI"

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  run:
    name: "tests & coverage"
    runs-on: ubuntu-latest
    strategy:
      matrix:
        python-version: ["3.7", "3.8", "3.9", "3.10", "3.11"]

    steps:
    - uses: actions/checkout@v3
    - name: Set up Python ${{ matrix.python-version }}
      uses: actions/setup-python@v4
      with:
        python-version: ${{ matrix.python-version }}

    - name: Install hatch
      run: python -m pip install hatch

    - name: Lint
      run: hatch run check

    - name: Coverage
      run: hatch run +py=${{ matrix.python-version }} test:test

    - name: Upload Coverage
      uses: codecov/codecov-action@v3
      with:
        files: coverage.xml
```

Hope this post can help you.
