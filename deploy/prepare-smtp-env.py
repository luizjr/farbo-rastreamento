#!/usr/bin/env python3

import os
import subprocess
import tempfile
from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent
COMPOSE = [
    "docker",
    "compose",
    "--env-file",
    ".env.production",
    "-f",
    "docker-compose.yml",
    "-f",
    "docker-compose.production.yml",
    "config",
    "--environment",
]


def main() -> None:
    result = subprocess.run(
        COMPOSE,
        cwd=ROOT,
        check=True,
        capture_output=True,
        text=True,
    )
    values = dict(
        line.split("=", 1)
        for line in result.stdout.splitlines()
        if "=" in line
    )
    password = values.get("SMTP_PASSWORD", "")
    if "\n" in password or "\r" in password:
        raise SystemExit("SMTP_PASSWORD must be a single-line value")

    fd, temp_name = tempfile.mkstemp(prefix=".env.smtp.", dir=ROOT)
    try:
        os.fchmod(fd, 0o600)
        with os.fdopen(fd, "w", encoding="utf-8") as temp_file:
            temp_file.write(f"SMTP_PASSWORD={password}\n")
        os.replace(temp_name, ROOT / ".env.smtp")
    except BaseException:
        try:
            os.unlink(temp_name)
        except FileNotFoundError:
            pass
        raise


if __name__ == "__main__":
    main()
