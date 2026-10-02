#!/usr/bin/env python3
"""Create opkg packages from staged files; no OpenWrt SDK is required for the Go build."""
import io, pathlib, subprocess, tarfile, time
ROOT=pathlib.Path(__file__).resolve().parent.parent
OUT=ROOT/'dist';OUT.mkdir(exist_ok=True)
VERSION='0.2.0'
def archive(files):
 b=io.BytesIO()
 with tarfile.open(fileobj=b,mode='w:gz') as t:
  for name,value,mode in files:
   v=value if isinstance(value,bytes) else value.encode()
   info=tarfile.TarInfo(name);info.size=len(v);info.mode=mode;info.mtime=int(time.time());t.addfile(info,io.BytesIO(v))
 return b.getvalue()
def write_ipk(path, members):
 """Write the tar.gz IPK container used by the target ImmortalWrt opkg."""
 with tarfile.open(path, mode='w:gz') as t:
  for name, value in members:
   value = value if isinstance(value, bytes) else value.encode()
   info = tarfile.TarInfo('./' + name)
   info.size = len(value)
   info.mode = 0o644
   info.mtime = int(time.time())
   t.addfile(info, io.BytesIO(value))
def package(name,arch,files,depends,conffiles=None):
 import tempfile, shutil
 with tempfile.TemporaryDirectory() as td:
  root=pathlib.Path(td); control=root/'control'; data=root/'data'; control.mkdir(); data.mkdir()
  (control/'control').write_text(f'Package: {name}\nVersion: {VERSION}\nArchitecture: {arch}\nMaintainer: Oterea\nSection: net\nPriority: optional\nDepends: {depends}\nDescription: sing-box-smart monitoring and service management\n')
  if conffiles:(control/'conffiles').write_text('\n'.join(conffiles)+'\n')
  if name.startswith('luci-app-'):(control/'postinst').write_text('#!/bin/sh\n[ -n "$IPKG_INSTROOT" ] && exit 0\n/etc/init.d/rpcd restart\nrm -f /tmp/luci-indexcache /tmp/luci-modulecache/*\nexit 0\n');(control/'postinst').chmod(0o755)
  for n,v,m in files:
   q=data/n;q.parent.mkdir(parents=True,exist_ok=True);q.write_bytes(v if isinstance(v,bytes) else v.encode());q.chmod(m)
  control_files=[]
  for q in control.rglob('*'):
   if q.is_file(): control_files.append((str(q.relative_to(control)), q.read_bytes(), q.stat().st_mode & 0o777))
  data_files=[]
  for q in data.rglob('*'):
   if q.is_file(): data_files.append((str(q.relative_to(data)), q.read_bytes(), q.stat().st_mode & 0o777))
  (root/'control.tar.gz').write_bytes(archive(control_files))
  (root/'data.tar.gz').write_bytes(archive(data_files))
  (root/'debian-binary').write_text('2.0\n')
  path=OUT/f'{name}_{VERSION}_{arch}.ipk'
  write_ipk(path, [('debian-binary',(root/'debian-binary').read_bytes()),('data.tar.gz',(root/'data.tar.gz').read_bytes()),('control.tar.gz',(root/'control.tar.gz').read_bytes())])
  print(path)
def collect(folder,prefix=''):
 return [(prefix+str(p.relative_to(folder)),p.read_bytes(),0o644) for p in folder.rglob('*') if p.is_file()]
subprocess.run(['go','build','-trimpath','-ldflags=-s -w','-o',str(OUT/'sing-box-smart'),'./cmd/sing-box-smart'],cwd=ROOT,check=True,env={**__import__('os').environ,'CGO_ENABLED':'0','GOOS':'linux','GOARCH':'arm64'})
package('sing-box-smart','aarch64_generic',[
 ('usr/bin/sing-box-smart',(OUT/'sing-box-smart').read_bytes(),0o755),
 ('etc/init.d/sing-box-smart',(ROOT/'deploy/sing-box-smart.init').read_bytes(),0o755),
 ('etc/sing-box-smart.conf',(ROOT/'deploy/sing-box-smart.conf').read_bytes(),0o600),
 ('lib/upgrade/keep.d/sing-box-smart',(ROOT/'deploy/sing-box-smart.keep').read_bytes(),0o644)],'libc',['/etc/sing-box-smart.conf'])
p=ROOT/'package/luci-app-sing-box-smart'
package('luci-app-sing-box-smart','all',collect(p/'root')+collect(p/'htdocs','www/'),'luci-base, sing-box-smart')
