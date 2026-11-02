#!/usr/bin/env python3
"""Check the regex manager extracts all explicitly pinned Makefile tools."""
import json
from pathlib import Path
import re

config = json.loads(Path('renovate.json').read_text(encoding='utf-8'))
pattern = config['customManagers'][0]['matchStrings'][0]
pattern = re.sub(r'\(\?<([A-Za-z]+)>', r'(?P<\1>', pattern)
found = {m.group('depName'): m.group('currentValue') for m in re.finditer(pattern, Path('Makefile').read_text(encoding='utf-8'))}
expected = {'sigs.k8s.io/kustomize/kustomize/v5', 'kubernetes-sigs/controller-tools',
            'golangci/golangci-lint', 'helm/helm', 'kubernetes-sigs/kind',
            'rhysd/actionlint', 'golang.org/x/vuln', 'yannh/kubeconform', 'goreleaser/goreleaser', 'renovate'}
missing = expected - found.keys()
assert not missing, f'Pinned tools not extracted: {missing}'
print(f'{len(found)} pinned tools extracted')
