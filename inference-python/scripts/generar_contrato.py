"""
Genera el código gRPC de Go y de Python a partir de proto/ (HU 05.4).

Usa el `protoc` que viene dentro de `grpcio-tools`, así que no hace falta
instalar protoc aparte. Para Go necesita los plugins (una sola vez):

    go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2

Uso (desde inference-python/):
    python -m scripts.generar_contrato              # genera Go y Python
    python -m scripts.generar_contrato --solo python
    python -m scripts.generar_contrato --verificar  # falla si el código generado no coincide con el .proto

Salida (se versiona en git, NO se edita a mano):
    inference-python/urbia/inferencia/v1/inferencia_pb2.py, _pb2.pyi, _pb2_grpc.py
    backend-go/internal/gen/inferenciav1/inferencia.pb.go, inferencia_grpc.pb.go
"""

from __future__ import annotations

import argparse
import filecmp
import os
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

RAIZ_PY = Path(__file__).resolve().parents[1]          # inference-python/
RAIZ_REPO = RAIZ_PY.parent
DIR_PROTO = RAIZ_REPO / "proto"
RAIZ_GO = RAIZ_REPO / "backend-go"
MODULO_GO = "github.com/PL290723/URBIA/backend-go"

SALIDA_PY = Path("urbia/inferencia/v1")                # relativa a la raíz de salida de Python
SALIDA_GO = Path("internal/gen/inferenciav1")          # relativa a backend-go/


def archivos_proto() -> list[str]:
    return sorted(p.relative_to(DIR_PROTO).as_posix() for p in DIR_PROTO.rglob("*.proto"))


def protoc(argumentos: list[str]) -> None:
    from grpc_tools import protoc as grpc_protoc
    import grpc_tools

    incluidos = Path(grpc_tools.__file__).parent / "_proto"   # tipos estándar de Google
    cmd = ["protoc", f"-I{DIR_PROTO}", f"-I{incluidos}", *argumentos, *archivos_proto()]
    if grpc_protoc.main(cmd) != 0:
        raise SystemExit("ERROR: protoc falló (revisa el mensaje de arriba)")


def plugin_go(nombre: str) -> Path:
    exe = nombre + (".exe" if os.name == "nt" else "")
    encontrado = shutil.which(exe)
    if encontrado:
        return Path(encontrado)
    try:
        gopath = subprocess.run(["go", "env", "GOPATH"], capture_output=True, text=True, check=True).stdout.strip()
    except (OSError, subprocess.CalledProcessError):
        gopath = ""
    candidato = Path(gopath) / "bin" / exe if gopath else None
    if candidato and candidato.is_file():
        return candidato
    raise SystemExit(
        f"ERROR: no encontré {exe}. Instálalo con:\n"
        "  go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12\n"
        "  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2"
    )


def generar_python(destino: Path) -> None:
    destino.mkdir(parents=True, exist_ok=True)  # protoc no crea la carpeta de salida
    protoc([f"--python_out={destino}", f"--pyi_out={destino}", f"--grpc_python_out={destino}"])


def generar_go(destino: Path) -> None:
    destino.mkdir(parents=True, exist_ok=True)
    protoc([
        f"--plugin=protoc-gen-go={plugin_go('protoc-gen-go')}",
        f"--plugin=protoc-gen-go-grpc={plugin_go('protoc-gen-go-grpc')}",
        f"--go_out={destino}", f"--go_opt=module={MODULO_GO}",
        f"--go-grpc_out={destino}", f"--go-grpc_opt=module={MODULO_GO}",
    ])


def comparar(generado: Path, versionado: Path) -> list[str]:
    if not versionado.is_dir():
        return [f"falta la carpeta {versionado}"]
    diferencias = []
    nombres = {p.name for p in generado.iterdir()} | {p.name for p in versionado.iterdir() if p.is_file()}
    for nombre in sorted(nombres):
        a, b = generado / nombre, versionado / nombre
        if not a.exists():
            diferencias.append(f"sobra {b}")
        elif not b.exists():
            diferencias.append(f"falta {b}")
        elif not filecmp.cmp(a, b, shallow=False):
            diferencias.append(f"desactualizado {b}")
    return diferencias


def main() -> int:
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--solo", choices=["python", "go"])
    p.add_argument("--verificar", action="store_true")
    args = p.parse_args()

    lenguajes = [args.solo] if args.solo else ["python", "go"]
    print(f"Protos: {', '.join(archivos_proto())}")

    if not args.verificar:
        if "python" in lenguajes:
            generar_python(RAIZ_PY)
            print(f"Python -> {RAIZ_PY / SALIDA_PY}")
        if "go" in lenguajes:
            generar_go(RAIZ_GO)
            print(f"Go     -> {RAIZ_GO / SALIDA_GO}")
        return 0

    diferencias = []
    with tempfile.TemporaryDirectory() as tmp:
        if "python" in lenguajes:
            generar_python(Path(tmp) / "py")
            diferencias += comparar(Path(tmp) / "py" / SALIDA_PY, RAIZ_PY / SALIDA_PY)
        if "go" in lenguajes:
            generar_go(Path(tmp) / "go")
            diferencias += comparar(Path(tmp) / "go" / SALIDA_GO, RAIZ_GO / SALIDA_GO)
    if diferencias:
        print("El código generado NO coincide con el .proto:")
        for d in diferencias:
            print(f"  - {d}")
        print("Ejecuta: python -m scripts.generar_contrato")
        return 1
    print("OK: el código generado coincide con el .proto")
    return 0


if __name__ == "__main__":
    sys.exit(main())