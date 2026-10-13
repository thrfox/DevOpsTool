"""测试、构建并打包当前平台的桌面发布成果物。"""

import argparse
from datetime import datetime
import os
from pathlib import Path
import re
import shutil
import subprocess
import tarfile
import tempfile
import zipfile


ROOT = Path(__file__).resolve().parents[1]


def run(*args, cwd=ROOT):
    subprocess.run(args, cwd=cwd, check=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--version', required=True)
    parser.add_argument('--platform', choices=['windows', 'linux', 'darwin'], required=True)
    parser.add_argument('--arch', choices=['amd64', 'arm64'], required=True)
    args = parser.parse_args()
    version = args.version.removeprefix('v')
    match = re.fullmatch(r'\d+\.\d+\.\d+\.(\d{10})(?:-[0-9A-Za-z.-]+)?', version)
    if not match:
        parser.error('版本必须类似 v0.0.1.2026101318 或 v0.0.1.2026101318-rc.1')
    try:
        datetime.strptime(match.group(1), '%Y%m%d%H')
    except ValueError:
        parser.error('版本时间必须是有效的年月日小时（上海时区）')
    # 原生构建，避免图形库与目标架构不匹配。
    actual_os = subprocess.check_output(['go', 'env', 'GOOS'], text=True).strip()
    actual_arch = subprocess.check_output(['go', 'env', 'GOARCH'], text=True).strip()
    if (actual_os, actual_arch) != (args.platform, args.arch):
        parser.error('运行环境与发布目标平台或架构不一致')
    os.environ['CGO_ENABLED'] = '1'
    output = ROOT / 'dist' / 'release'
    output.mkdir(parents=True, exist_ok=True)
    stem = f'DevOpsTool-v{version}-{args.platform}-{args.arch}'
    if args.platform == 'windows':
        run('powershell', '-NoProfile', '-ExecutionPolicy', 'Bypass', '-File',
            str(ROOT / 'scripts/build.ps1'), '-Test', '-Version', version)
        binary = ROOT / 'dist/DevOpsTool.exe'
        # 防止误发布依赖额外 MinGW 运行库的程序。
        imports = subprocess.check_output(['objdump', '-p', str(binary)], text=True)
        dlls = re.findall(r'DLL Name:\s*(\S+)', imports, re.IGNORECASE)
        if not dlls or any(dll.lower().startswith(('libgcc', 'libstdc++', 'libwinpthread')) for dll in dlls):
            raise RuntimeError('Windows 程序动态库检查失败')
        print('Windows 动态库：', ', '.join(dlls))
        shutil.copy2(binary, output / f'{stem}.exe')
    else:
        run('go', 'run', './scripts/icons')
        if args.platform == 'linux':
            run('xvfb-run', '-a', 'go', 'test', './...')
        else:
            run('go', 'test', './...')
        binary = ROOT / 'dist/DevOpsTool'
        run('go', 'build', '-trimpath', '-ldflags',
            f'-s -w -X devopstool/internal/buildinfo.Version={version}',
            '-o', str(binary), './cmd/devopstool')
    with tempfile.TemporaryDirectory(dir=ROOT / 'dist') as temporary:
        stage = Path(temporary)
        if args.platform == 'darwin':
            # 使用固定版本的官方打包工具生成带图标的 macOS 应用包。
            fyne = Path(subprocess.check_output(['go', 'env', 'GOPATH'], text=True).strip()) / 'bin/fyne'
            run(str(fyne), 'package', '-os', 'darwin', '-name', 'DevOpsTool',
                '-appID', 'cn.azukin.devopstool', '-appVersion', version.split('-')[0].rsplit('.', 1)[0],
                '-appBuild', os.environ.get('GITHUB_RUN_NUMBER', '1'),
                '-icon', str(ROOT / 'internal/ui/assets/app.png'),
                '-executable', str(binary), cwd=stage)
            app = stage / 'DevOpsTool.app'
            if not app.is_dir():
                raise RuntimeError('未生成 macOS 应用包')
            run('codesign', '--force', '--deep', '--sign', '-', str(app))
            run('codesign', '--verify', '--deep', '--strict', str(app))
            # 本地临时签名用于应用结构校验，不是开发者证书签名或公证。
        else:
            shutil.copy2(binary, stage / binary.name)
        for filename in ('LICENSE', 'THIRD_PARTY.md', 'README.md'):
            shutil.copy2(ROOT / filename, stage / filename)
        shutil.copy2(ROOT / 'internal/ui/assets/OFL.txt', stage / 'FONT_LICENSE.txt')
        shutil.copy2(ROOT / 'internal/ui/assets/THIRD_PARTY_LICENSES.txt', stage / 'THIRD_PARTY_LICENSES.txt')
        if args.platform == 'windows':
            with zipfile.ZipFile(output / f'{stem}.zip', 'w', zipfile.ZIP_DEFLATED) as archive:
                for file in sorted(stage.iterdir()):
                    archive.write(file, file.name)
        else:
            # tar 保留应用包内的可执行权限和符号链接。
            with tarfile.open(output / f'{stem}.tar.gz', 'w:gz') as archive:
                for file in sorted(stage.iterdir()):
                    archive.add(file, arcname=file.name)
    print('成果物目录：', output)


if __name__ == '__main__':
    main()
