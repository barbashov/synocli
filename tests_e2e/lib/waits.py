from __future__ import annotations

import time
from typing import Callable, TypeVar

T = TypeVar("T")


class WaitTimeout(AssertionError):
    pass


def wait_for(
    predicate: Callable[[], T],
    timeout: float = 60.0,
    interval: float = 1.0,
    description: str = "condition",
) -> T:
    """Poll `predicate` until it returns a truthy value, or raise WaitTimeout.

    The truthy return value is propagated so callers can do, e.g.,
        result = wait_for(lambda: fetch() if ready() else None)
    """
    deadline = time.monotonic() + timeout
    last_value: T | None = None
    while True:
        value = predicate()
        if value:
            return value
        last_value = value
        if time.monotonic() >= deadline:
            raise WaitTimeout(
                f"timeout after {timeout}s waiting for {description} "
                f"(last value: {last_value!r})"
            )
        time.sleep(interval)
