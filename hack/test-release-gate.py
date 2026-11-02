#!/usr/bin/env python3
"""Exercise the CI release gate against a fake GitHub API response."""
import os
from pathlib import Path
import subprocess
import tempfile
import textwrap

workflow = Path('.github/workflows/ci.yml').read_text(encoding='utf-8')
step = workflow.split('      - name: Check that main has not advanced\n', 1)[1]
script = textwrap.dedent(step.split('        run: |\n', 1)[1].split('\n      - name:', 1)[0])
with tempfile.TemporaryDirectory() as directory:
    temp = Path(directory)
    fake_gh = temp / 'gh'
    fake_gh.write_text('#!/bin/sh\nif [ "$FAKE_FAIL" = true ]; then exit 1; fi\nprintf "%s\\n" "$FAKE_MAIN_SHA"\n', encoding='utf-8')
    fake_gh.chmod(0o755)
    cases = [('same', 'same', 'false', 0, 'current=true'),
             ('old', 'new', 'false', 0, 'current=false'),
             ('same', 'same', 'true', 1, None)]
    for index, (commit, current, fail, expected_status, expected_output) in enumerate(cases):
        output = temp / f'output-{index}'
        env = {**os.environ, 'PATH': f'{temp}:{os.environ["PATH"]}', 'GITHUB_REPOSITORY': 'example/repo',
               'GITHUB_SHA': commit, 'GITHUB_OUTPUT': str(output), 'FAKE_MAIN_SHA': current, 'FAKE_FAIL': fail}
        result = subprocess.run(['bash', '-e', '-c', script], env=env, capture_output=True, check=False)
        assert result.returncode == expected_status, result.stderr.decode()
        if expected_output:
            assert output.read_text(encoding='utf-8').strip() == expected_output
        else:
            assert not output.exists(), 'failed API calls must not enable the release job'
print(f'{len(cases)} release gate cases passed')
