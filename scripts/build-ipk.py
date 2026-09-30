#!/usr/bin/env python3
"""Create opkg packages from staged files; no OpenWrt SDK is required for the Go build."""
import io, json, pathlib, subprocess, tarfile, time
ROOT=pathlib.Path(__file__).resolve().parent.parent
OUT=ROOT/'dist';OUT.mkdir(exist_ok=True)
VERSION='0.2.0'
def archive(files):
 b=io.BytesIO()
 with tarfile.open(fileobj=b,mode='w:gz') as t:
  for name,value,mode in files:
   v=value if isinstance(value,bytes) else value.encode()
   info=tarfile.TarInfo('./'+name);info.size=len(v);info.mode=mode;info.mtime=int(time.time());t.addfile(info,io.BytesIO(v))
 return b.getvalue()
def package(name,arch,files,depends,conffiles=None):
 control=f'Package: {name}\nVersion: {VERSION}\nArchitecture: {arch}\nMaintainer: Oterea\nSection: net\nPriority: optional\nDepends: {depends}\nDescription: sing-box-smart monitoring and service management\n'
 controls=[('control',control,0o644)]
 if conffiles:controls.append(('conffiles','\n'.join(conffiles)+'\n',0o644))
 if name.startswith('luci-app-'):
  controls.append(('postinst','#!/bin/sh\n[ -n "$IPKG_INSTROOT" ] && exit 0\n/etc/init.d/rpcd restart\nrm -f /tmp/luci-indexcache /tmp/luci-modulecache/*\nexit 0\n',0o755))
 members=[('debian-binary',b'2.0\n'),('control.tar.gz',archive(controls)),('data.tar.gz',archive(files))]
 b=bytearray(b'!<arch>\n')
 for key,v in members:
  header=f'{key+"/":<16}{int(time.time()):<12}{0:<6}{0:<6}{"100644":<8}{len(v):<10}`\n'.encode();b.extend(header);b.extend(v)
  if len(v)%2:b.extend(b'\n')
 path=OUT/f'{name}_{VERSION}_{arch}.ipk';path.write_bytes(b);print(path)
def collect(folder,prefix=''):
 return [(prefix+str(p.relative_to(folder)),p.read_bytes(),0o644) for p in folder.rglob('*') if p.is_file()]
subprocess.run(['go','build','-trimpath','-ldflags=-s -w','-o',str(OUT/'sing-box-smart'),'./cmd/sing-box-smart'],cwd=ROOT,check=True,env={**__import__('os').environ,'CGO_ENABLED':'0','GOOS':'linux','GOARCH':'arm64'})
package('sing-box-smart','aarch64_generic',[
 ('usr/bin/sing-box-smart',(OUT/'sing-box-smart').read_bytes(),0o755),
 ('etc/init.d/sing-box-smart',(ROOT/'deploy/sing-box-smart.init').read_bytes(),0o755),
 ('etc/sing-box-smart.conf',(ROOT/'deploy/sing-box-smart.conf').read_bytes(),0o600),
 ('lib/upgrade/keep.d/sing-box-smart','/etc/sing-box-smart/\n/etc/sing-box-smart.conf\n',0o644)],'libc',['/etc/sing-box-smart.conf'])
p=ROOT/'package/luci-app-sing-box-smart'
package('luci-app-sing-box-smart','all',collect(p/'root')+collect(p/'htdocs','www/'),'luci-base, sing-box-smart')
