"""Give each LuCI release distinct module and stylesheet URLs."""
import json
import re


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
            data = data.replace(b'sing-box-smart.settings as serviceSettings', f'sing-box-smart.{release}.settings as serviceSettings'.encode())
            data = data.replace(b'sing-box-smart/style.css', f'sing-box-smart/{release}/style.css'.encode())
        files.append(('www/' + name, data, source.stat().st_mode & 0o777))
    return files
