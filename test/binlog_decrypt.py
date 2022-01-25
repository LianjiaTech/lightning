#!/usr/bin/env python3

# Copy from: https://mysql.wisborg.dk/2019/01/28/automatic-decryption-of-mysql-binary-logs-using-python/

import sys
import os
import struct
import collections
import hashlib
from cryptography.hazmat.primitives.ciphers import Cipher, algorithms, modes
from cryptography.hazmat.backends import default_backend

def key_and_iv_from_password(password):
    # Based on
    # https://stackoverflow.com/questions/13907841/implement-openssl-aes-encryption-in-python

    key_length = 32
    iv_length = 16
    required_length = key_length + iv_length
    password = password

    key_iv = hashlib.sha512(password).digest()
    tmp = [key_iv]
    while len(tmp) < required_length:
        tmp.append(hashlib.sha512(tmp[-1] + password).digest())
        key_iv += tmp[-1]

    key = key_iv[:key_length]
    iv = key_iv[key_length:required_length]

    return key, iv


class Key(
    collections.namedtuple(
        'Key', [
            'key_id',
            'key_type',
            'user_id',
            'key_data',
        ]
    )):
    __slots__ = ()


class Keyring(object):
    _keys = []
    _keyring_file_version = None
    _xor_str = '*305=Ljt0*!@$Hnm(*-9-w;:'.encode('utf-8')

    def __init__(self, keyring_filepath):
        self.read_keyring(keyring_filepath)

    def _read_key(self, data):
        overall_length = struct.unpack('<Q', data[0:8])[0]
        key_id_length = struct.unpack('<Q', data[8:16])[0]
        key_type_length = struct.unpack('<Q', data[16:24])[0]
        user_id_length = struct.unpack('<Q', data[24:32])[0]
        key_length = struct.unpack('<Q', data[32:40])[0]

        key_id_start = 40
        key_type_start = key_id_start + key_id_length
        user_id_start = key_type_start + key_type_length
        key_start = user_id_start + user_id_length
        key_end = key_start + key_length

        key_id = data[key_id_start:key_type_start].decode('utf-8')
        key_type = data[key_type_start:user_id_start].decode('utf-8')
        # The User ID may be blank in which case the length is zero
        user_id = data[user_id_start:key_start].decode('utf-8') if user_id_length > 0 else None
        key_raw = data[key_start:key_end]
        xor_str_len = len(self._xor_str)
        key_data = bytes([key_raw[i] ^ self._xor_str[i%xor_str_len]
                          for i in range(len(key_raw))])

        return Key(key_id, key_type, user_id, key_data)

    def read_keyring(self, filepath):
        keyring_data = bytearray()
        with open(filepath, 'rb') as keyring_fs:
            chunk = keyring_fs.read()
            while len(chunk) > 0:
                keyring_data.extend(chunk)
                chunk = keyring_fs.read()

            keyring_fs.close()

        # Verify the start of the file is "Keyring file version:"
        header = keyring_data[0:21]
        if header.decode('utf-8') != 'Keyring file version:':
            raise ValueError('Invalid header in the keyring file: {0}'
                             .format(header.hex()))

        # Get the keyring version - currently only 2.0 is supported
        version = keyring_data[21:24].decode('utf-8')
        if version != '2.0':
            raise ValueError('Unsupported keyring version: {0}'
                             .format(version))

        self._keyring_file_version = version
        keyring_length = len(keyring_data)
        offset = 24
        keys = []
        while offset < keyring_length and keyring_data[offset:offset+3] != b'EOF':
            key_length = struct.unpack('<Q', keyring_data[offset:offset+8])[0]
            key_data = keyring_data[offset:offset+key_length]
            key = self._read_key(key_data)
            keys.append(key)
            offset += key_length

        self._keys = keys

    def get_key(self, key_id, user_id):
        for key in self._keys:
            if key.key_id == key_id and key.user_id == user_id:
                return key

        return None


def decrypt_binlog(binlog, keyring, out_dir, prefix):
    '''Decrypts a binary log and outputs it to out_dir with the prefix
    prepended. The arguments are:

        * binlog - the path to the encrypted binary log
        * keyring - a Keyring object
        * out_dir - the output directory
        * prefix - prefix to add to the binary log basename.
    '''
    magic_encrypted = 'fd62696e'
    magic_decrypted = 'fe62696e'

    binlog_basename = os.path.basename(binlog)
    decrypt_binlog_path = os.path.join(
        out_dir, '{0}{1}'.format(prefix, binlog_basename))
    if os.path.exists(decrypt_binlog_path):
        print("{0}: Decrypted binary log path, '{1}' already exists. Skipping"
              .format(binlog_basename, decrypt_binlog_path), file=sys.stderr)
        return False

    with open(binlog, 'rb') as binlog_fs:
        # Verify the magic bytes are correct
        magic = binlog_fs.read(4)
        if magic.hex() == magic_decrypted:
            print('{0}: Binary log is not encrypted. Skipping.'
                  .format(binlog_basename), file=sys.stderr)
            return False
        elif magic.hex() != magic_encrypted:
            print("{0}: Found invalid magic '0x{1}' for encrypted binlog file."
                  .format(binlog_basename, magic.hex(), file=sys.stderr))
            return False

        # Get the encrypted version (must currently be 1)
        version = struct.unpack('<B', binlog_fs.read(1))[0]
        if version != 1:
            print("{0}: Unsupported binary log encrypted version '{1}'"
                  .format(binlog_basename, version), file=sys.stderr)
            return False

        # First header field is a TLV: the keyring key ID
        field_type = struct.unpack('<B', binlog_fs.read(1))[0]
        if field_type != 1:
            print('{0}: Invalid field type ({1}). Keyring key ID (1) was '
                  + 'expected.'.format(binlog_basename, field_type),
                  file=sys.stderr)
            return False

        keyring_id_len = struct.unpack('<B', binlog_fs.read(1))[0]
        keyring_id = binlog_fs.read(keyring_id_len).decode('utf-8')
        print("{0}: Keyring key ID for is '{1}'"
              .format(binlog_basename, keyring_id), file=sys.stderr)

        # Get the key from the keyring file
        key = keyring.get_key(keyring_id, None)

        # Second header is a TV: the encrypted file password
        field_type = struct.unpack('<B', binlog_fs.read(1))[0]
        if field_type != 2:
            print('{0}: Invalid field type ({1}). Encrypted file password (2) '
                  + 'was expected.'.format(binlog_basename, field_type),
                  file=sys.stderr)
            return False
        encrypted_password = binlog_fs.read(32)

        # Third header field is a TV: the IV to decrypt the file password
        field_type = struct.unpack('<B', binlog_fs.read(1))[0]
        if field_type != 3:
            print('{0}: Invalid field type ({1}). IV to decrypt the file '
                  + 'password (3) was expected.'
                  .format(binlog_basename, field_type), file=sys.stderr)
            return False
        iv = binlog_fs.read(16)
        backend = default_backend()
        cipher = Cipher(algorithms.AES(key.key_data), modes.CBC(iv),
                        backend=backend)
        decryptor = cipher.decryptor()
        password = decryptor.update(encrypted_password) + decryptor.finalize()

        # Generate the file key and IV
        key, iv = key_and_iv_from_password(password)
        nonce = iv[0:8] + bytes(8)
     
        # Decrypt the file data (the binary log content)
        # The encrypted binary log headers are 512, so skip those
        binlog_fs.seek(512, os.SEEK_SET)
        binlog_encrypted_data = binlog_fs.read()
        binlog_fs.close()

    cipher = Cipher(algorithms.AES(key), modes.CTR(nonce), backend=backend)
    decryptor = cipher.decryptor()
    binlog_decrypted_data = decryptor.update(binlog_encrypted_data)
    binlog_decrypted_data += decryptor.finalize()
    binlog_encrypted_data = None

    # Check decrypted binary log magic
    magic = binlog_decrypted_data[0:4]
    if magic.hex() != magic_decrypted:
        print("{0}: Found invalid magic '0x{1}' for decrypted binlog file."
              .format(binlog_basename, magic.hex()), file=sys.stderr)
        return False

    # Write the decrypted binary log to disk
    with open(decrypt_binlog_path, 'wb') as new_fs:
        new_fs.write(binlog_decrypted_data)
        new_fs.close()

    print("{0}: Successfully decrypted as '{1}'"
          .format(binlog_basename, decrypt_binlog_path))
    return True

def decrypt_binlogs(args):
    '''Outer routine for decrypted one or more binary logs. The
    argument args is a named touple (typically from the argparse
    parser) with the following members:

       * args.binlogs - a list or tuple of the binary logs to decrypt
       * args.keyring_file_data - the path to the file with the
            kerying data for the keyring_file plugin.
       * args.dir - the output directory for the decrypted binary logs
       * args.prefix - the prefix to prepend to the basename of the
            encrypted binary log filenames. This allows you to output
            the decrypted to the same directory as the encrypted
            binary logs without overwriting the original files.
    '''
    keyring = Keyring(args.keyring_file_data)
    for binlog in args.binlogs:
        decrypt_binlog(binlog, keyring, args.dir, args.prefix)

def main(argv):
    import argparse

    parser = argparse.ArgumentParser(
        prog='decrypt_binlog.py',
        description='Decrypt one or more binary log files from MySQL Server '
                   +'8.0.14+ created with binlog_encryption = ON. The '
                   +'binary log files have the prefix given with --prefix '
                   +'prepended to their file names.'
                   +'If an output file already exists, the file will be '
                   +'skipped.',
        epilog='All work is performed in-memory. For this reason, the'
               +'expected peak memory usage is around three times the'
               +'size of the largest binary log. As max_binlog_size can'
               +'at most be 1G, for instances exlusively executing small'
               +'transactions, the memory usage can thus be up to around'
               +'3.5G. For instances executing large transactions, the'
               +'binary log files can be much larger than 1G and thus the'
               +'memory usage equally larger.')

    parser.add_argument('-d', '--dir', default=os.getcwd(),
        dest='dir',
        help='The destination directory for the decrypted binary log files. '
             +'The default is to use the current directory.')

    parser.add_argument('-p', '--prefix', default='plain-',
        dest='prefix',
        help='The prefix to prepand to the basename of the binary log file.'
             +'The default is plain-.')

    parser.add_argument('-k', '--keyring_file_data', default=None,
        dest='keyring_file_data',
        help='The path to the keyring file. The same as keyring_file_data in '
             +'the MySQL configuration. This option is mandatory.')

    parser.add_argument('binlogs', nargs=argparse.REMAINDER,
                        help='The binary log files to decrypt.')

    args = parser.parse_args()
    if not args.binlogs:
        print('ERROR: At least one binary log file must be specified.\n',
              file=sys.stderr)
        parser.print_help(file=sys.stderr)
        sys.exit(1)

    if not args.keyring_file_data:
        print('ERROR: The path to the keyring file must be specified.\n',
              file=sys.stderr)
        parser.print_help(file=sys.stderr)
        sys.exit(1)

    decrypt_binlogs(args)


if __name__ == '__main__':
   main(sys.argv[1:])
