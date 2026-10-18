"""Build IPK tar members with explicit parent directories for opkg."""
import io
import pathlib
import tarfile
import time


def archive(files):
    output = io.BytesIO()
    directories = set()
    with tarfile.open(fileobj=output, mode='w:gz') as tar:
        for name, value, mode in files:
            for parent in reversed(pathlib.PurePosixPath(name).parents):
                directory = str(parent)
                if directory == '.' or directory in directories:
                    continue
                info = tarfile.TarInfo(directory)
                info.type = tarfile.DIRTYPE
                info.mode = 0o755
                info.mtime = int(time.time())
                tar.addfile(info)
                directories.add(directory)
            content = value if isinstance(value, bytes) else value.encode()
            info = tarfile.TarInfo(name)
            info.size = len(content)
            info.mode = mode
            info.mtime = int(time.time())
            tar.addfile(info, io.BytesIO(content))
    return output.getvalue()
