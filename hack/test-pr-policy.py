#!/usr/bin/env python3
"""Exercise the actual inline PR-policy script without repository write privileges."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import textwrap

workflow = Path('.github/workflows/pr-policy.yml').read_text(encoding='utf-8')
script = textwrap.dedent(workflow.split("node <<'JS'\n", 1)[1].split('\n          JS', 1)[0])
cases = [
    ([], 'feat: example', False),
    (['feature'], 'feat: example', True),
    (['feature', 'bug'], 'fix: example', False),
    (['feature', 'autorelease: pending'], 'feat: example', True),
    (['ci', 'autorelease: pending'], 'chore(main): release 0.1.0', True),
    (['dependencies'], 'not conventional', False),
]
with tempfile.TemporaryDirectory() as directory:
    event = Path(directory) / 'event.json'
    for labels, title, expected in cases:
        event.write_text(json.dumps({'pull_request': {'labels': [{'name': x} for x in labels], 'title': title}}))
        result = subprocess.run(['node', '-e', script], env={**os.environ, 'EVENT_PATH': str(event)}, capture_output=True, check=False)
        assert (result.returncode == 0) == expected, (labels, title, result.stderr.decode())
print(f'{len(cases)} PR policy cases passed')
