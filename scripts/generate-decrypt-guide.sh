#!/usr/bin/env bash
set -euo pipefail

# generate-decrypt-guide.sh
# 生成客户解密指南 README-DECRYPT.txt（支持中文+英文）

VERSION_SCRIPT="1.0.0"

usage() {
    cat <<'USAGE'
Usage: generate-decrypt-guide.sh [OPTIONS]

Generate a decryption guide (README-DECRYPT.txt) for encrypted release files.

Required options:
  --filename NAME          Original filename (e.g. release-v2.4.1.zip)
  --sha256 HASH            SHA-256 hash of the original file
  --sha256-encrypted HASH  SHA-256 hash of the encrypted file
  --algo ALGO              Encryption algorithm (e.g. AES-256)
  --version VER            Release version (e.g. v2.4.1)

Optional:
  --lang LANGS             Languages to include: zh, en, zh+en (default: zh+en)
  --output FILE            Output file path (default: README-DECRYPT.txt)
  --help                   Show this help message

Examples:
  ./generate-decrypt-guide.sh \
    --filename release-v2.4.1.zip \
    --sha256 e3b0c44... \
    --sha256-encrypted abc123... \
    --algo AES-256 \
    --version v2.4.1 \
    --lang zh+en \
    --output README-DECRYPT.txt
USAGE
    exit 0
}

die() { echo "ERROR: $*" >&2; exit 1; }

# --- Defaults ---
FILENAME=""
SHA256=""
SHA256_ENCRYPTED=""
ALGO=""
RELEASE_VERSION=""
LANG_OPT="zh+en"
OUTPUT="README-DECRYPT.txt"

# --- Parse arguments ---
if ! OPTS=$(getopt -o h --long help,filename:,sha256:,sha256-encrypted:,algo:,version:,lang:,output: -n 'generate-decrypt-guide.sh' -- "$@"); then
    die "Failed to parse arguments. Use --help for usage."
fi
eval set -- "$OPTS"

while true; do
    case "$1" in
        --filename)         FILENAME="$2";         shift 2 ;;
        --sha256)           SHA256="$2";           shift 2 ;;
        --sha256-encrypted) SHA256_ENCRYPTED="$2"; shift 2 ;;
        --algo)             ALGO="$2";             shift 2 ;;
        --version)          RELEASE_VERSION="$2";  shift 2 ;;
        --lang)             LANG_OPT="$2";         shift 2 ;;
        --output)           OUTPUT="$2";           shift 2 ;;
        -h|--help)          usage ;;
        --)                 shift; break ;;
        *)                  die "Unknown option: $1" ;;
    esac
done

# --- Validate required params ---
[[ -z "$FILENAME" ]]         && die "Missing required option: --filename"
[[ -z "$SHA256" ]]           && die "Missing required option: --sha256"
[[ -z "$SHA256_ENCRYPTED" ]] && die "Missing required option: --sha256-encrypted"
[[ -z "$ALGO" ]]             && die "Missing required option: --algo"
[[ -z "$RELEASE_VERSION" ]]  && die "Missing required option: --version"

# --- Determine languages ---
INCLUDE_ZH=false
INCLUDE_EN=false
case "$LANG_OPT" in
    zh)    INCLUDE_ZH=true ;;
    en)    INCLUDE_EN=true ;;
    zh+en|en+zh) INCLUDE_ZH=true; INCLUDE_EN=true ;;
    *) die "Unsupported --lang value: $LANG_OPT (use zh, en, or zh+en)" ;;
esac

DATE_NOW=$(date '+%Y-%m-%d')

# --- Template rendering helper ---
render() {
    sed \
        -e "s|{{FILENAME}}|${FILENAME}|g" \
        -e "s|{{SHA256}}|${SHA256}|g" \
        -e "s|{{SHA256_ENCRYPTED}}|${SHA256_ENCRYPTED}|g" \
        -e "s|{{ALGO}}|${ALGO}|g" \
        -e "s|{{VERSION}}|${RELEASE_VERSION}|g" \
        -e "s|{{DATE}}|${DATE_NOW}|g"
}

# --- Generate Chinese section ---
generate_zh() {
    cat <<'TPL'
================================================================================
                          文件解密指南
================================================================================

版本: {{VERSION}}
日期: {{DATE}}
加密算法: {{ALGO}}

--------------------------------------------------------------------------------
一、文件信息
--------------------------------------------------------------------------------

  文件名:           {{FILENAME}}
  加密文件:         {{FILENAME}}.gpg
  原始文件 SHA-256: {{SHA256}}
  加密文件 SHA-256: {{SHA256_ENCRYPTED}}

--------------------------------------------------------------------------------
二、解密步骤
--------------------------------------------------------------------------------

  请确保您已收到解密密钥文件 session-key.txt。

  【Linux / macOS】

  1. 安装 GPG（如尚未安装）:
     - Ubuntu/Debian:  sudo apt-get install gnupg
     - CentOS/RHEL:    sudo yum install gnupg2
     - macOS:          brew install gnupg

  2. 执行解密:
     gpg --batch --yes --passphrase-file session-key.txt \
         --decrypt --output {{FILENAME}} {{FILENAME}}.gpg

  【Windows】

  方法一: 命令行（需安装 Gpg4win）

  1. 下载并安装 Gpg4win: https://www.gpg4win.org/
  2. 打开命令提示符或 PowerShell，执行:
     gpg --batch --yes --passphrase-file session-key.txt ^
         --decrypt --output {{FILENAME}} {{FILENAME}}.gpg

  方法二: Kleopatra 图形界面

  1. 安装 Gpg4win（包含 Kleopatra）
  2. 双击 {{FILENAME}}.gpg 文件，Kleopatra 将自动打开
  3. 在弹出的密码对话框中，输入 session-key.txt 中的密钥内容
  4. 选择输出路径，点击"解密"

--------------------------------------------------------------------------------
三、SHA-256 校验
--------------------------------------------------------------------------------

  解密完成后，请校验文件完整性。

  【Linux】
  sha256sum -c {{FILENAME}}.sha256

  或手动比对:
  sha256sum {{FILENAME}}
  # 预期值: {{SHA256}}

  【macOS】
  shasum -a 256 -c {{FILENAME}}.sha256

  或手动比对:
  shasum -a 256 {{FILENAME}}
  # 预期值: {{SHA256}}

  【Windows】
  certutil -hashfile {{FILENAME}} SHA256
  # 将输出与预期值比对: {{SHA256}}

--------------------------------------------------------------------------------
四、常见问题 (FAQ)
--------------------------------------------------------------------------------

  Q: 提示 "decryption failed: Bad session key" 怎么办?
  A: 请确认 session-key.txt 文件内容完整，没有多余的空行或空格。
     可用以下命令检查: cat -A session-key.txt

  Q: 提示 "No such file or directory" 怎么办?
  A: 请确认 .gpg 文件和 session-key.txt 在当前目录下，
     或使用完整路径指定文件位置。

  Q: SHA-256 校验不通过怎么办?
  A: 文件可能在传输过程中损坏，请重新下载加密文件后再次解密。
     如问题持续，请联系技术支持。

  Q: Windows 上 gpg 命令找不到?
  A: 请确认 Gpg4win 已正确安装，并将其添加到系统 PATH 中。
     默认安装路径: C:\Program Files (x86)\GnuPG\bin

  Q: 如何验证加密文件本身的完整性?
  A: 加密文件 SHA-256: {{SHA256_ENCRYPTED}}
     使用上述校验方法对 {{FILENAME}}.gpg 进行校验。

TPL
}

# --- Generate English section ---
generate_en() {
    cat <<'TPL'
================================================================================
                        File Decryption Guide
================================================================================

Version: {{VERSION}}
Date: {{DATE}}
Encryption Algorithm: {{ALGO}}

--------------------------------------------------------------------------------
1. File Information
--------------------------------------------------------------------------------

  Filename:                {{FILENAME}}
  Encrypted file:          {{FILENAME}}.gpg
  Original file SHA-256:   {{SHA256}}
  Encrypted file SHA-256:  {{SHA256_ENCRYPTED}}

--------------------------------------------------------------------------------
2. Decryption Steps
--------------------------------------------------------------------------------

  Make sure you have received the decryption key file: session-key.txt

  [Linux / macOS]

  1. Install GPG (if not already installed):
     - Ubuntu/Debian:  sudo apt-get install gnupg
     - CentOS/RHEL:    sudo yum install gnupg2
     - macOS:          brew install gnupg

  2. Decrypt the file:
     gpg --batch --yes --passphrase-file session-key.txt \
         --decrypt --output {{FILENAME}} {{FILENAME}}.gpg

  [Windows]

  Option A: Command line (requires Gpg4win)

  1. Download and install Gpg4win: https://www.gpg4win.org/
  2. Open Command Prompt or PowerShell and run:
     gpg --batch --yes --passphrase-file session-key.txt ^
         --decrypt --output {{FILENAME}} {{FILENAME}}.gpg

  Option B: Kleopatra GUI

  1. Install Gpg4win (includes Kleopatra)
  2. Double-click the {{FILENAME}}.gpg file; Kleopatra will open automatically
  3. Enter the passphrase from session-key.txt when prompted
  4. Choose the output path and click "Decrypt"

--------------------------------------------------------------------------------
3. SHA-256 Verification
--------------------------------------------------------------------------------

  After decryption, verify file integrity.

  [Linux]
  sha256sum -c {{FILENAME}}.sha256

  Or manually compare:
  sha256sum {{FILENAME}}
  # Expected: {{SHA256}}

  [macOS]
  shasum -a 256 -c {{FILENAME}}.sha256

  Or manually compare:
  shasum -a 256 {{FILENAME}}
  # Expected: {{SHA256}}

  [Windows]
  certutil -hashfile {{FILENAME}} SHA256
  # Compare output with expected: {{SHA256}}

--------------------------------------------------------------------------------
4. FAQ
--------------------------------------------------------------------------------

  Q: "decryption failed: Bad session key" error?
  A: Verify that session-key.txt is complete with no extra blank lines or
     trailing spaces. Check with: cat -A session-key.txt

  Q: "No such file or directory" error?
  A: Ensure the .gpg file and session-key.txt are in the current directory,
     or specify full paths.

  Q: SHA-256 verification failed?
  A: The file may have been corrupted during transfer. Re-download the
     encrypted file and decrypt again. Contact support if the issue persists.

  Q: gpg command not found on Windows?
  A: Ensure Gpg4win is installed and added to the system PATH.
     Default path: C:\Program Files (x86)\GnuPG\bin

  Q: How to verify the encrypted file itself?
  A: Encrypted file SHA-256: {{SHA256_ENCRYPTED}}
     Use the verification methods above on {{FILENAME}}.gpg.

TPL
}

# --- Build output ---
{
    if $INCLUDE_ZH; then
        generate_zh | render
    fi
    if $INCLUDE_ZH && $INCLUDE_EN; then
        echo ""
        echo "========================================================================"
        echo ""
    fi
    if $INCLUDE_EN; then
        generate_en | render
    fi
} > "$OUTPUT"

echo "Decryption guide generated: $OUTPUT"
