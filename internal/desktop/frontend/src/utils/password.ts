const PASSWORD_ALPHABET = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789'

export function generatePassword(length = 32) {
  let password = ''
  while (password.length < length) {
    const bytes = new Uint8Array(Math.max(48, length - password.length))
    crypto.getRandomValues(bytes)
    for (const byte of bytes) {
      // 248 is the largest multiple of the 62-character alphabet below 256.
      if (byte < 248) password += PASSWORD_ALPHABET[byte % PASSWORD_ALPHABET.length]
      if (password.length === length) break
    }
  }
  return password
}
