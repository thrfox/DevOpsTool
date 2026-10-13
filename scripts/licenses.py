"""将模块许可证嵌入程序，便于单文件发布。"""
import json
import os
import pathlib
import subprocess

root = pathlib.Path(__file__).resolve().parents[1]
env = os.environ.copy()
env['GOPATH'] = str(root / '.tools/gopath')
go = root / '.tools/go/bin/go.exe'
output = subprocess.check_output([str(go), 'list', '-m', '-json', 'all'], cwd=root, env=env).decode('utf-8')
decoder = json.JSONDecoder()
modules = []
while output.strip():
    output = output.lstrip()
    item, offset = decoder.raw_decode(output)
    modules.append(item)
    output = output[offset:]

parts = ['DevOpsTool 第三方许可证与版权声明\n']
for module in modules:
    if module.get('Main') or not module.get('Dir'):
        continue
    directory = pathlib.Path(module['Dir'])
    files = [p for p in directory.iterdir() if p.is_file() and p.name.lower().startswith(('license', 'copying', 'notice', 'copyright'))]
    parts.append('\n' + '=' * 72 + '\n' + module['Path'] + ' ' + module.get('Version', '') + '\n')
    if not files:
        # 子模块可能继承仓库父目录的许可证。
        for parent in directory.parents:
            if parent == root / '.tools/gopath/pkg/mod':
                break
            files = [p for p in parent.glob('*') if p.is_file() and p.name.lower().startswith(('license', 'copying', 'notice', 'copyright'))]
            if files:
                break
    for file in files:
        parts.append('\n' + file.name + '\n' + file.read_text(encoding='utf-8', errors='replace') + '\n')
parts.append('\nNoto Sans SC\n' + (root / 'internal/ui/assets/OFL.txt').read_text(encoding='utf-8'))
(root / 'internal/ui/assets/THIRD_PARTY_LICENSES.txt').write_text(''.join(parts), encoding='utf-8')
print('已生成第三方许可证清单')
