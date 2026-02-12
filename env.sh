#!/bin/bash

[ -d venv ] || python3 -m venv venv

source venv/bin/activate

echo "Upgrading pip:"
pip install -U pip

echo "Installing dependencies from requirements.txt:"
pip install -r requirements.txt

echo "Environment ready. You can now run 'mkdocs serve'."