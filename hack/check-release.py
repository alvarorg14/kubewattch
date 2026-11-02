#!/usr/bin/env python3
"""Check the single-release metadata and operator component inventory."""
import json
from pathlib import Path
import re

version = Path('VERSION').read_text(encoding='utf-8').strip()
chart = Path('charts/kubewattch/Chart.yaml').read_text(encoding='utf-8')
for key in ('version', 'appVersion'):
    match = re.search(rf'^{key}:\s*["\']?([^"\'\s]+)', chart, re.MULTILINE)
    assert match and match.group(1) == version, f'{key} differs from VERSION'
config = json.loads(Path('release-please-config.json').read_text(encoding='utf-8'))
root = config['packages']['.']
assert root['release-type'] == 'simple' and root['version-file'] == 'VERSION'
assert root['extra-files'] == [{'type': 'yaml', 'path': 'charts/kubewattch/Chart.yaml', 'jsonpath': '$[version,appVersion]'}]
assert json.loads(Path('.release-please-manifest.json').read_text(encoding='utf-8')).get('.', version) == version
inventory = json.loads(Path('build/components.json').read_text(encoding='utf-8'))['include']
assert [entry['name'] for entry in inventory] == ['operator']
for entry in inventory:
    assert Path(entry['dockerfile']).is_file()
assert Path('build/operator/Dockerfile.goreleaser').is_file()
assert '{{ .Env.OPERATOR_IMAGE }}' in Path('.goreleaser.yaml').read_text(encoding='utf-8')
print(f'Release metadata and {len(inventory)} component verified at {version}')
