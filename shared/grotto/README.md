# grotto: *G*ithub *R*equest *Auto*mation

This directory contains some silly Python code that is used in terra-helmfile GitHub Actions, and by Jenkins scripts that interact with terra-helmfile. Perhaps we'll rewrite in Go at some point.

## Developer Setup

On OSX:

    brew install python

    pip3 install --user virtualenv

Add the following line to shell initialization files:

    export PATH="$PATH:${HOME}/Library/Python/3.9/bin"

After cloning the git repo, change into the grotto directory and:

    cd ~/projects/terra-helmfile-images/shared/grotto

    pipenv install --dev

## Running Unit Tests

With coverage report:

    pipenv run coverage run --source=./grotto -m pytest

Generate HTML report from results and view in browser:

    pipenv run coverage html

    open htmlcov/index.html

Without coverage report:

    pipenv run pytest
