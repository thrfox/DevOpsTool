"""仅在项目内下载便携编译器，不改变系统配置。"""
import hashlib
import json
import pathlib
import urllib.request
import zipfile

ROOT = pathlib.Path(__file__).resolve().parents[1]
TOOLS = ROOT / '.tools'
TOOLS.mkdir(exist_ok=True)

def fetch(url, target):
    print('下载：' + url, flush=True)
    request = urllib.request.Request(url, headers={'User-Agent': 'DevOpsTool-build'})
    with urllib.request.urlopen(request, timeout=120) as source, target.open('wb') as out:
        while chunk := source.read(1024 * 1024):
            out.write(chunk)

if not (TOOLS / 'go/bin/go.exe').exists():
    with urllib.request.urlopen('https://go.dev/dl/?mode=json', timeout=30) as response:
        releases = json.load(response)
    archive = next(f for f in releases[0]['files'] if f['os'] == 'windows' and f['arch'] == 'amd64' and f['kind'] == 'archive')
    target = TOOLS / 'go.zip'
    fetch('https://go.dev/dl/' + archive['filename'], target)
    if hashlib.sha256(target.read_bytes()).hexdigest() != archive['sha256']:
        raise RuntimeError('Go 下载校验失败')
    with zipfile.ZipFile(target) as package:
        package.extractall(TOOLS)
    print('Go 编译器准备完成', flush=True)

if not (TOOLS / 'mingw64/bin/gcc.exe').exists():
    request = urllib.request.Request('https://api.github.com/repos/brechtsanders/winlibs_mingw/releases/latest', headers={'User-Agent': 'DevOpsTool-build'})
    with urllib.request.urlopen(request, timeout=30) as response:
        release = json.load(response)
    asset = next(a for a in release['assets'] if a['name'].startswith('winlibs-x86_64-posix-seh-gcc-') and 'llvm' not in a['name'] and 'ucrt' in a['name'] and a['name'].endswith('.zip'))
    url = asset['browser_download_url']
    target = TOOLS / 'mingw.zip'
    fetch(url, target)
    if asset.get('digest', '').startswith('sha256:') and hashlib.sha256(target.read_bytes()).hexdigest() != asset['digest'][7:]:
        raise RuntimeError('C 编译器下载校验失败')
    with zipfile.ZipFile(target) as package:
        package.extractall(TOOLS)
    print('C 编译器准备完成', flush=True)
