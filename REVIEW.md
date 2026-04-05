# Review Guidelines — process-mcp

Inherits from org-wide [REVIEW.md](https://github.com/hairglasses-studio/.github/blob/main/REVIEW.md).

## Additional Focus
- **PID reuse races**: PIDs can be recycled — verify process identity before signaling
- **Signal handling**: Only allow safe signals (SIGTERM, SIGINT) by default, require confirmation for SIGKILL
- **Composed investigation**: The "tool-of-tools" pattern must bound output size
- **Port scanning**: Validate port ranges (1-65535), handle permission errors on privileged ports
