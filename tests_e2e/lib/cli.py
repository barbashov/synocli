from __future__ import annotations

import json
import os
import subprocess
import sys
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Iterable, Optional


class CLIError(AssertionError):
    """Raised when a synocli invocation that was expected to succeed did not.

    Carries the original CLIResult plus the stderr of an automatic --debug
    retry (when one was performed) so the failure message is self-contained.
    """

    def __init__(self, message: str, result: "CLIResult", debug_stderr: str = "") -> None:
        super().__init__(message)
        self.result = result
        self.debug_stderr = debug_stderr


@dataclass
class CLIResult:
    args: list[str]
    exit_code: int
    stdout: str
    stderr: str
    envelope: Optional[dict] = None
    debug_stderr: str = ""

    def envelope_or_raise(self) -> dict:
        if self.envelope is None:
            raise AssertionError(
                f"command produced no JSON envelope; stdout={self.stdout!r} stderr={self.stderr!r}"
            )
        return self.envelope

    def synology_code(self) -> Optional[int]:
        env = self.envelope or {}
        err = env.get("error") or {}
        details = err.get("details") or {}
        code = details.get("synology_code")
        return code if isinstance(code, int) else None


@dataclass
class SynoCLI:
    bin_path: Path
    endpoint: str
    credentials_file: Optional[Path]
    insecure_tls: bool
    invocation_log: Optional[Path] = None
    extra_args: list[str] = field(default_factory=list)

    def _base_args(self) -> list[str]:
        args: list[str] = [str(self.bin_path), "--endpoint", self.endpoint]
        if self.credentials_file is not None:
            args.extend(["--credentials-file", str(self.credentials_file)])
        if self.insecure_tls:
            args.append("--insecure-tls")
        args.extend(self.extra_args)
        return args

    def build_cmd(self, *args: str) -> list[str]:
        """Return the full argv for streaming/Popen callers that can't use run()."""
        return self._base_args() + list(args)

    def with_credentials(self, credentials_file: Path) -> "SynoCLI":
        return SynoCLI(
            bin_path=self.bin_path,
            endpoint=self.endpoint,
            credentials_file=credentials_file,
            insecure_tls=self.insecure_tls,
            invocation_log=self.invocation_log,
            extra_args=list(self.extra_args),
        )

    def _log_invocation(self, args: Iterable[str]) -> None:
        if not self.invocation_log:
            return
        try:
            self.invocation_log.parent.mkdir(parents=True, exist_ok=True)
            with self.invocation_log.open("a", encoding="utf-8") as fh:
                # Strip transport-only args so the log shows the user-visible
                # command shape, matching how the bash scripts logged ">>> ..."
                redacted: list[str] = []
                skip_next = False
                for a in args:
                    if skip_next:
                        skip_next = False
                        continue
                    if a in ("--endpoint", "--credentials-file"):
                        skip_next = True
                        continue
                    if a == "--insecure-tls":
                        continue
                    redacted.append(a)
                fh.write(" ".join(redacted) + "\n")
        except OSError:
            # Logging is best-effort; never let it break a test.
            pass

    def run(
        self,
        *args: str,
        json_output: bool = True,
        check: bool = True,
        timeout: float = 120.0,
        stdin: Optional[str] = None,
        retry_with_debug: bool = True,
    ) -> CLIResult:
        cmd = self._base_args() + list(args)
        if json_output and "--json" not in cmd:
            cmd.append("--json")
        self._log_invocation(cmd)
        proc = subprocess.run(
            cmd,
            input=stdin,
            capture_output=True,
            text=True,
            timeout=timeout,
        )
        envelope: Optional[dict] = None
        if json_output and proc.stdout:
            try:
                envelope = json.loads(proc.stdout)
            except json.JSONDecodeError:
                envelope = None
        result = CLIResult(
            args=list(args),
            exit_code=proc.returncode,
            stdout=proc.stdout,
            stderr=proc.stderr,
            envelope=envelope,
        )

        if check and proc.returncode != 0:
            debug_stderr = ""
            if retry_with_debug and "--debug" not in cmd:
                debug_cmd = cmd + ["--debug"]
                self._log_invocation(debug_cmd)
                debug_proc = subprocess.run(
                    debug_cmd,
                    input=stdin,
                    capture_output=True,
                    text=True,
                    timeout=timeout,
                )
                debug_stderr = debug_proc.stderr
                result.debug_stderr = debug_stderr
            raise CLIError(
                self._format_failure(cmd, result),
                result,
                debug_stderr=debug_stderr,
            )
        return result

    def run_expect_failure(
        self,
        *args: str,
        exit_code: Optional[int] = None,
        code: Optional[str] = None,
        timeout: float = 120.0,
        stdin: Optional[str] = None,
    ) -> CLIResult:
        result = self.run(*args, check=False, timeout=timeout, stdin=stdin)
        if result.exit_code == 0:
            raise AssertionError(
                f"expected command to fail but it succeeded: {' '.join(args)}\n"
                f"stdout={result.stdout!r}"
            )
        if exit_code is not None and result.exit_code != exit_code:
            raise AssertionError(
                f"expected exit code {exit_code}, got {result.exit_code} "
                f"for: {' '.join(args)}\nstdout={result.stdout!r}\nstderr={result.stderr!r}"
            )
        if code is not None:
            env = result.envelope or {}
            err = env.get("error") or {}
            actual_code = err.get("code")
            if actual_code != code:
                raise AssertionError(
                    f"expected envelope error code {code!r}, got {actual_code!r} "
                    f"for: {' '.join(args)}\nenvelope={env!r}"
                )
        return result

    @staticmethod
    def _format_failure(cmd: list[str], result: CLIResult) -> str:
        shown = " ".join(cmd)
        lines = [
            f"synocli command failed (exit {result.exit_code}): {shown}",
            f"stdout: {result.stdout.strip()}",
            f"stderr: {result.stderr.strip()}",
        ]
        if result.debug_stderr:
            lines.append("--debug retry stderr:")
            lines.append(result.debug_stderr.strip())
        return "\n".join(lines)


def assert_envelope_ok(result: CLIResult) -> dict:
    env = result.envelope_or_raise()
    for key in ("ok", "command", "meta"):
        if key not in env:
            raise AssertionError(f"envelope missing {key!r}: {env!r}")
    if env.get("ok") is not True:
        raise AssertionError(f"expected ok=true, got {env.get('ok')!r}: {env!r}")
    return env


def envelope_data(result: CLIResult) -> dict:
    env = assert_envelope_ok(result)
    data = env.get("data") or {}
    if not isinstance(data, dict):
        raise AssertionError(f"envelope.data is not a dict: {data!r}")
    return data
