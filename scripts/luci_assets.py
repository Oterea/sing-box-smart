"""Give each LuCI release distinct module and stylesheet URLs."""
import json
import re
import shutil
import sys
from pathlib import Path


def collect_luci_files(package_root, version):
    if not re.fullmatch(r'[0-9A-Za-z._+-]+', version):
        raise ValueError('Invalid package version')
    release = 'v' + re.sub(r'[^0-9A-Za-z_]', '_', version)
    files = []
    for source in sorted((package_root / 'root').rglob('*')):
        if not source.is_file():
            continue
        name = str(source.relative_to(package_root / 'root'))
        data = source.read_bytes()
        if name.endswith('/menu.d/luci-app-sing-box-smart.json'):
            menu = json.loads(data)
            menu['admin/services/sing-box-smart']['action']['path'] = f'sing-box-smart/{release}/app'
            data = (json.dumps(menu, ensure_ascii=False) + '\n').encode()
        files.append((name, data, source.stat().st_mode & 0o777))
    for source in sorted((package_root / 'htdocs').rglob('*')):
        if not source.is_file():
            continue
        name = str(source.relative_to(package_root / 'htdocs'))
        name = name.replace('view/sing-box-smart/', f'view/sing-box-smart/{release}/')
        name = name.replace('resources/sing-box-smart/', f'resources/sing-box-smart/{release}/')
        data = source.read_bytes()
        if source.name == 'app.js':
            data = data.replace(b'sing-box-smart.coreconfig as coreConfig', f'sing-box-smart.{release}.coreconfig as coreConfig'.encode())
            data = data.replace(b'sing-box-smart/style.css', f'sing-box-smart/{release}/style.css'.encode())
        files.append(('www/' + name, data, source.stat().st_mode & 0o777))
    return files


def stage_luci_files(package_root, version):
    """Rewrite a LuCI package tree with versioned runtime asset paths."""
    package_root = Path(package_root)
    staged = package_root.parent / (package_root.name + '.staged')
    if staged.exists():
        shutil.rmtree(staged)
    (staged / 'root').mkdir(parents=True)
    (staged / 'htdocs').mkdir(parents=True)
    for name, data, mode in collect_luci_files(package_root, version):
        if name.startswith('www/'):
            target = staged / 'htdocs' / name.removeprefix('www/')
        else:
            target = staged / 'root' / name
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(data)
        target.chmod(mode)
    for directory in ('root', 'htdocs'):
        original = package_root / directory
        shutil.rmtree(original)
        shutil.move(str(staged / directory), str(original))
    staged.rmdir()


if __name__ == '__main__':
    if len(sys.argv) != 4 or sys.argv[1] != 'stage':
        raise SystemExit('usage: luci_assets.py stage PACKAGE_ROOT VERSION')
    stage_luci_files(sys.argv[2], sys.argv[3])
