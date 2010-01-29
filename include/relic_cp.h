/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007, 2008, 2009 RELIC Authors
 *
 * This file is part of RELIC. RELIC is legal property of its developers,
 * whose names are not listed here. Please refer to the COPYRIGHT file
 * for contact information.
 *
 * RELIC is free software; you can redistribute it and/or
 * modify it under the terms of the GNU Lesser General Public
 * License as published by the Free Software Foundation; either
 * version 2.1 of the License, or (at your option) any later version.
 *
 * RELIC is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with RELIC. If not, see <http://www.gnu.org/licenses/>.
 */

/**
 * @defgroup cp Cryptographic protocols.
 */

/**
 * @file
 *
 * Interface of cryptographic protocols.
 *
 * @version $Id$
 * @ingroup bn
 */

#ifndef RELIC_CP_H
#define RELIC_CP_H

#include "relic_conf.h"
#include "relic_types.h"
#include "relic_bn.h"
#include "relic_ec.h"
#include "relic_pc.h"

/*============================================================================*/
/* Type definitions.                                                          */
/*============================================================================*/

/**
 * Represents an RSA key pair.
 */
typedef struct _rsa_t {
	/** The modulus n = pq. */
	bn_t n;
	/** The public exponent. */
	bn_t e;
	/** The private exponent. */
	bn_t d;
	/** The first prime p. */
	bn_t p;
	/** The second prime q. */
	bn_t q;
	/** The inverse of e modulo (p-1). */
	bn_t dp;
	/** The inverse of e modulo (q-1). */
	bn_t dq;
	/** The inverse of q modulo p. */
	bn_t qi;
} rsa_t;

/**
 * Represents a Rabin key pair.
 */
typedef struct _rabin_t {
	/** The modulus n = pq. */
	bn_t n;
	/** The first prime p. */
	bn_t p;
	/** The second prime q. */
	bn_t q;
	/** The cofactor of the first prime. */
	bn_t dp;
	/** The cofactor of the second prime. */
	bn_t dq;
} rabin_t;

/**
 * Represents a SOK key pair.
 */
typedef struct _sokaka {
	/** The private key in G_1. */
	g1_t s1;
	/** The private key in G_2. */
	g2_t s2;
} sokaka_t;

/*============================================================================*/
/* Macro definitions                                                          */
/*============================================================================*/

/**
 * Generates a new RSA key pair.
 *
 * @param[out] PB			- the public key.
 * @param[out] PV			- the private key.
 * @param[in] B				- the key length in bits.
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
#if CP_RSA == BASIC
#define cp_rsa_gen(PB, PV, B)				cp_rsa_gen_basic(PB, PV, B)
#elif CP_RSA == QUICK
#define cp_rsa_gen(PB, PV, B)				cp_rsa_gen_quick(PB, PV, B)
#endif

/**
 * Decrypts using RSA.
 *
 * @param[out] O			- the output buffer.
 * @param[out] OL			- the number of bytes written in the output buffer.
 * @param[in] I				- the input buffer.
 * @param[in] IL			- the number of bytes to encrypt.
 * @param[in] P				- the private key.
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
#if CP_RSA == BASIC
#define cp_rsa_dec(O, OL, I, IL, P)			cp_rsa_dec_basic(O, OL, I, IL, P)
#elif CP_RSA == QUICK
#define cp_rsa_dec(O, OL, I, IL, P)			cp_rsa_dec_quick(O, OL, I, IL, P)
#endif

/**
 * Signs a message using the RSA cryptosystem.
 * @param[out] O			- the output buffer.
 * @param[out] OL			- the number of bytes written in the output buffer.
 * @param[in] I				- the input buffer.
 * @param[in] IL			- the number of bytes to sign.
 * @param[in] P				- the private key.
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
#if CP_RSA == BASIC
#define cp_rsa_sign(O, OL, I, IL, P)		cp_rsa_sign_basic(O, OL, I, IL, P)
#elif CP_RSA == QUICK
#define cp_rsa_sign(O, OL, I, IL, P)		cp_rsa_sign_quick(O, OL, I, IL, P)
#endif

/*============================================================================*/
/* Function prototypes                                                        */
/*============================================================================*/

/**
 * Generates a new key pair for basic RSA algorithm.
 *
 * @param[out] pub			- the public key.
 * @param[out] prv			- the private key.
 * @param[in] bits			- the key length in bits.
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
int cp_rsa_gen_basic(rsa_t pub, rsa_t prv, int bits);

/**
 * Generates a new key RSA pair for fast operations with the CRT optimization.
 *
 * @param[out] pub			- the public key.
 * @param[out] prv			- the private key.
 * @param[in] bits			- the key length in bits.
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
int cp_rsa_gen_quick(rsa_t pub, rsa_t prv, int bits);

/**
 * Encrypts using the RSA cryptosystem.
 *
 * @param[out] out			- the output buffer.
 * @param[out] out_len		- the number of bytes written in the output buffer.
 * @param[in] in			- the input buffer.
 * @param[in] in_len		- the number of bytes to encrypt.
 * @param[in] pub			- the public key.
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
int cp_rsa_enc(unsigned char *out, int *out_len, unsigned char *in, int in_len,
		rsa_t pub);

/**
 * Decrypts using the basic RSA decryption method.
 *
 * @param[out] out			- the output buffer.
 * @param[out] out_len		- the number of bytes written in the output buffer.
 * @param[in] in			- the input buffer.
 * @param[in] in_len		- the number of bytes to encrypt.
 * @param[in] prv			- the private key.
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
int cp_rsa_dec_basic(unsigned char *out, int *out_len, unsigned char *in,
		int in_len, rsa_t prv);

/**
 * Decrypts using the fast RSA decryption with CRT optimization.
 *
 * @param[out] out			- the output buffer.
 * @param[out] out_len		- the number of bytes written in the output buffer.
 * @param[in] in			- the input buffer.
 * @param[in] in_len		- the number of bytes to encrypt.
 * @param[in] prv			- the private key.
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
int cp_rsa_dec_quick(unsigned char *out, int *out_len, unsigned char *in,
		int in_len, rsa_t prv);

/**
 * Signs using the basic RSA signature algorithm.
 *
 * @param[out] sig			- the signature
 * @param[out] sig_len		- the number of bytes written in the signature.
 * @param[in] msg			- the message to sign.
 * @param[in] msg_len		- the number of bytes to sign.
 * @param[in] prv			- the private key.
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
int cp_rsa_sign_basic(unsigned char *sig, int *sig_len, unsigned char *msg,
		int msg_len, rsa_t prv);

/**
 * Signs using the fast RSA signature algorithm with CRT optimization.
 *
 * @param[out] sig			- the signature
 * @param[out] sig_len		- the number of bytes written in the signature.
 * @param[in] msg			- the message to sign.
 * @param[in] msg_len		- the number of bytes to sign.
 * @param[in] prv			- the private key.
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
int cp_rsa_sign_quick(unsigned char *sig, int *sig_len, unsigned char *msg,
		int msg_len, rsa_t prv);

/**
 * Verifies an RSA signature.
 *
 * @param[in] sig			- the signature to verify.
 * @param[in] sig_len		- the signature length in bytes.
 * @param[in] msg			- the signed message.
 * @param[in] msg_len		- the message length in bytes.
 * @param[in] pub			- the public key.
 * @return 1 if the signature is valid, 0 otherwise.
 */
int cp_rsa_ver(unsigned char *sig, int sig_len, unsigned char *msg, int msg_len,
		rsa_t pub);

/**
 * Generates a new key pair for the Rabin cryptosystem.
 *
 * @param[out] pub			- the public key.
 * @param[out] prv			- the private key,
 * @param[in] bits			- the key length in bits.
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
int cp_rabin_gen(rabin_t pub, rabin_t prv, int bits);

/**
 * Encrypts using the Rabin cryptosystem.
 *
 * @param[out] out			- the output buffer.
 * @param[out] out_len		- the number of bytes written in the output buffer.
 * @param[in] in			- the input buffer.
 * @param[in] in_len		- the number of bytes to encrypt.
 * @param[in] pub			- the public key.
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
int cp_rabin_enc(unsigned char *out, int *out_len, unsigned char *in, int in_len,
		rabin_t pub);

/**
 * Decrypts using the Rabin cryptosystem.
 *
 * @param[out] out			- the output buffer.
 * @param[out] out_len		- the number of bytes written in the output buffer.
 * @param[in] in			- the input buffer.
 * @param[in] in_len		- the number of bytes to encrypt.
 * @param[in] prv			- the private key.
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
int cp_rabin_dec(unsigned char *out, int *out_len, unsigned char *in, int in_len,
		rabin_t prv);

/**
 * Generates an ECDSA key pair.
 *
 * @param[out] d			- the private key.
 * @param[in] q				- the public key.
 */
void cp_ecdsa_gen(bn_t d, ec_t q);

/**
 * Signs a message using ECDSA.
 *
 * @param[out] r				- the first component of the signature.
 * @param[out] s				- the second component of the signature.
 * @param[in] msg				- the message to sign.
 * @param[in] len				- the message length in bytes.
 * @param[in] d					- the private key.
 */
void cp_ecdsa_sign(bn_t r, bn_t s, unsigned char *msg, int len, bn_t d);

/**
 * Verifies a message signed with ECDSA using the basic method.
 *
 * @param[out] r				- the first component of the signature.
 * @param[out] s				- the second component of the signature.
 * @param[in] msg				- the message to sign.
 * @param[in] len				- the message length in bytes.
 * @param[in] q					- the public key.
 */
int cp_ecdsa_ver(bn_t r, bn_t s, unsigned char *msg, int len, ec_t q);

/**
 * Generates a master key for the SOK identity-based non-interactive
 * authenticated key agreement protocol.
 *
 * @param[out] master			- the master key.
 */
void cp_sokaka_gen(bn_t master);

/**
 * Generates a private key for the SOK protocol.
 *
 * @param[out] k				- the private key.
 * @param[in] id				- the identity.
 * @param[in] len				- the length of identity in bytes.
 * @param[in] master			- the master key.
 */
void cp_sokaka_gen_prv(sokaka_t k, char *id, int len, bn_t master);

/**
 * Computes a shared key between two entities.
 *
 * @param[out] key				- the shared key.
 * @param[int] key_len			- the intended shared key length in bytes.
 * @param[in] id1				- the first identity.
 * @param[in] len1				- the length of the first identity in bytes.
 * @param[in] k					- the private key of the first identity.
 * @param[in] id2				- the second identity.
 * @param[in] len2				- the length of the second identity in bytes.
 */
void cp_sokaka_key(unsigned char *key, unsigned int key_len, char *id1, int len1, sokaka_t k, char *id2, int len2);

#endif /* !RELIC_CP_H */
