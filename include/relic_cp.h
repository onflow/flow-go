/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2014 RELIC Authors
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
 * @defgroup cp Cryptographic protocols
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
/* Constant definitions                                                       */
/*============================================================================*/

/**
 * Flag used to indicate that the message being signed is plaintext.
 */
#define CP_TEXT		0

/**
 * Flag used to indicate that the message being signed is a hash value.
 */
#define CP_HASH		1

/**
 * Flag used to indicate that no padding should be used.
 */
#define CP_EMPTY	0

/**
 * Flag used to indicate that PKCS#1 v1.5 padding should be used.
 */
#define CP_PKCS1	1

/**
 * Flag used to indicate that PKCS#1 v2.5 padding should be used.
 */
#define CP_PKCS2	2

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
} rsa_st;

/**
 * Pointer to an RSA key pair.
 */
#if ALLOC == AUTO
typedef rsa_st rsa_t[1];
#else
typedef rsa_st *rsa_t;
#endif

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
} rabin_st;

/**
 * Pointer to a Rabin key pair.
 */
#if ALLOC == AUTO
typedef rabin_st rabin_t[1];
#else
typedef rabin_st *rabin_t;
#endif

/**
 * Represents a Benaloh's Dense Probabilistic Encryption key pair.
 */
typedef struct _bdpe_t {
	/** The modulus n = pq. */
	bn_t n;
	/** The first prime p. */
	bn_t p;
	/** The second prime q. */
	bn_t q;
	/** The random element in {0, ..., n - 1}. */
	bn_t y;
	/** The divisor of (p-1) such that gcd(t, (p-1)/t) = gcd(t, q-1) = 1. */
	dig_t t;
} bdpe_st;

/**
 * Pointer to a Benaloh's Dense Probabilistic Encryption key pair.
 */
#if ALLOC == AUTO
typedef bdpe_st bdpe_t[1];
#else
typedef bdpe_st *bdpe_t;
#endif

/**
 * Represents a SOK key pair.
 */
typedef struct _sokaka {
	/** The private key in G_1. */
	g1_t s1;
	/** The private key in G_2. */
	g2_t s2;
} sokaka_st;

/**
 * Pointer to SOK key pair.
 */
#if ALLOC == AUTO
typedef sokaka_st sokaka_t[1];
#else
typedef sokaka_st *sokaka_t;
#endif

/*============================================================================*/
/* Macro definitions                                                          */
/*============================================================================*/

/**
 * Initializes an RSA key pair with a null value.
 *
 * @param[out] A			- the key pair to initialize.
 */
#if ALLOC == AUTO
#define rsa_null(A)				/* empty */
#else
#define rsa_null(A)			A = NULL;
#endif

/**
 * Calls a function to allocate and initialize an RSA key pair.
 *
 * @param[out] A			- the new key pair.
 */
#if ALLOC == DYNAMIC
#define rsa_new(A)															\
	A = (rsa_t)calloc(1, sizeof(rsa_st));									\
	if (A == NULL) {														\
		THROW(ERR_NO_MEMORY);												\
	}																		\
	bn_null((A)->e);														\
	bn_null((A)->n);														\
	bn_null((A)->d);														\
	bn_null((A)->dp);														\
	bn_null((A)->dq);														\
	bn_null((A)->p);														\
	bn_null((A)->q);														\
	bn_null((A)->qi);														\
	bn_new((A)->e);															\
	bn_new((A)->n);															\
	bn_new((A)->d);															\
	bn_new((A)->dp);														\
	bn_new((A)->dq);														\
	bn_new((A)->p);															\
	bn_new((A)->q);															\
	bn_new((A)->qi);														\

#elif ALLOC == STATIC
#define rsa_new(A)															\
	A = (rsa_t)alloca(sizeof(rsa_st));										\
	if (A == NULL) {														\
		THROW(ERR_NO_MEMORY);												\
	}																		\
	bn_null((A)->e);														\
	bn_null((A)->n);														\
	bn_null((A)->d);														\
	bn_null((A)->dp);														\
	bn_null((A)->dq);														\
	bn_null((A)->p);														\
	bn_null((A)->q);														\
	bn_null((A)->qi);														\
	bn_new((A)->e);															\
	bn_new((A)->n);															\
	bn_new((A)->d);															\
	bn_new((A)->dp);														\
	bn_new((A)->dq);														\
	bn_new((A)->p);															\
	bn_new((A)->q);															\
	bn_new((A)->qi);														\

#elif ALLOC == AUTO
#define rsa_new(A)															\
	bn_new((A)->e);															\
	bn_new((A)->n);															\
	bn_new((A)->d);															\
	bn_new((A)->dp);														\
	bn_new((A)->dq);														\
	bn_new((A)->p);															\
	bn_new((A)->q);															\
	bn_new((A)->qi);														\

#elif ALLOC == STACK
#define rsa_new(A)															\
	A = (rsa_t)alloca(sizeof(rsa_st));										\
	bn_new((A)->e);															\
	bn_new((A)->n);															\
	bn_new((A)->d);															\
	bn_new((A)->dp);														\
	bn_new((A)->dq);														\
	bn_new((A)->p);															\
	bn_new((A)->q);															\
	bn_new((A)->qi);														\

#endif

/**
 * Calls a function to clean and free an RSA key pair.
 *
 * @param[out] A			- the key pair to clean and free.
 */
#if ALLOC == DYNAMIC
#define rsa_free(A)															\
	if (A != NULL) {														\
		bn_free((A)->e);													\
		bn_free((A)->n);													\
		bn_free((A)->d);													\
		bn_free((A)->dp);													\
		bn_free((A)->dq);													\
		bn_free((A)->p);													\
		bn_free((A)->q);													\
		bn_free((A)->qi);													\
		free(A);															\
		A = NULL;															\
	}

#elif ALLOC == STATIC
#define rsa_free(A)															\
	if (A != NULL) {														\
		bn_free((A)->e);													\
		bn_free((A)->n);													\
		bn_free((A)->d);													\
		bn_free((A)->dp);													\
		bn_free((A)->dq);													\
		bn_free((A)->p);													\
		bn_free((A)->q);													\
		bn_free((A)->qi);													\
		A = NULL;															\
	}																		\

#elif ALLOC == AUTO
#define rsa_free(A)				/* empty */

#elif ALLOC == STACK
#define rsa_free(A)															\
	bn_free((A)->e);														\
	bn_free((A)->n);														\
	bn_free((A)->d);														\
	bn_free((A)->dp);														\
	bn_free((A)->dq);														\
	bn_free((A)->p);														\
	bn_free((A)->q);														\
	bn_free((A)->qi);														\
	A = NULL;																\

#endif

/**
 * Initializes a Rabin key pair with a null value.
 *
 * @param[out] A			- the key pair to initialize.
 */
#if ALLOC == AUTO
#define rabin_null(A)			/* empty */
#else
#define rabin_null(A)		A = NULL;
#endif

/**
 * Calls a function to allocate and initialize a Rabin key pair.
 *
 * @param[out] A			- the new key pair.
 */
#if ALLOC == DYNAMIC
#define rabin_new(A)														\
	A = (rabin_t)calloc(1, sizeof(rabin_st));								\
	if (A == NULL) {														\
		THROW(ERR_NO_MEMORY);												\
	}																		\
	bn_new((A)->n);															\
	bn_new((A)->dp);														\
	bn_new((A)->dq);														\
	bn_new((A)->p);															\
	bn_new((A)->q);															\

#elif ALLOC == STATIC
#define rabin_new(A)														\
	A = (rabin_t)alloca(sizeof(rabin_st));									\
	if (A == NULL) {														\
		THROW(ERR_NO_MEMORY);												\
	}																		\
	bn_new((A)->n);															\
	bn_new((A)->dp);														\
	bn_new((A)->dq);														\
	bn_new((A)->p);															\
	bn_new((A)->q);															\

#elif ALLOC == AUTO
#define rabin_new(A)														\
	bn_new((A)->n);															\
	bn_new((A)->dp);														\
	bn_new((A)->dq);														\
	bn_new((A)->p);															\
	bn_new((A)->q);															\

#elif ALLOC == STACK
#define rabin_new(A)														\
	A = (rabin_t)alloca(sizeof(rabin_st));									\
	bn_new((A)->n);															\
	bn_new((A)->dp);														\
	bn_new((A)->dq);														\
	bn_new((A)->p);															\
	bn_new((A)->q);															\

#endif

/**
 * Calls a function to clean and free a Rabin key pair.
 *
 * @param[out] A			- the key pair to clean and free.
 */
#if ALLOC == DYNAMIC
#define rabin_free(A)														\
	if (A != NULL) {														\
		bn_free((A)->n);													\
		bn_free((A)->dp);													\
		bn_free((A)->dq);													\
		bn_free((A)->p);													\
		bn_free((A)->q);													\
		free(A);															\
		A = NULL;															\
	}

#elif ALLOC == STATIC
#define rabin_free(A)														\
	if (A != NULL) {														\
		bn_free((A)->n);													\
		bn_free((A)->dp);													\
		bn_free((A)->dq);													\
		bn_free((A)->p);													\
		bn_free((A)->q);													\
		A = NULL;															\
	}																		\

#elif ALLOC == AUTO
#define rabin_free(A)			/* empty */

#elif ALLOC == STACK
#define rabin_free(A)														\
	bn_free((A)->n);														\
	bn_free((A)->dp);														\
	bn_free((A)->dq);														\
	bn_free((A)->p);														\
	bn_free((A)->q);														\
	A = NULL;																\

#endif

/**
 * Initializes a Benaloh's key pair with a null value.
 *
 * @param[out] A			- the key pair to initialize.
 */
#if ALLOC == AUTO
#define bdpe_null(A)			/* empty */
#else
#define bdpe_null(A)			A = NULL;
#endif

/**
 * Calls a function to allocate and initialize a Benaloh's key pair.
 *
 * @param[out] A			- the new key pair.
 */
#if ALLOC == DYNAMIC
#define bdpe_new(A)															\
	A = (bdpe_t)calloc(1, sizeof(bdpe_st));									\
	if (A == NULL) {														\
		THROW(ERR_NO_MEMORY);												\
	}																		\
	bn_new((A)->n);															\
	bn_new((A)->y);															\
	bn_new((A)->p);															\
	bn_new((A)->q);															\
	(A)->t = 0;																\

#elif ALLOC == STATIC
#define bdpe_new(A)															\
	A = (bdpe_t)alloca(sizeof(bdpe_st));									\
	if (A == NULL) {														\
		THROW(ERR_NO_MEMORY);												\
	}																		\
	bn_new((A)->n);															\
	bn_new((A)->y);															\
	bn_new((A)->p);															\
	bn_new((A)->q);															\
	(A)->t = 0;																\

#elif ALLOC == AUTO
#define bdpe_new(A)															\
	bn_new((A)->n);															\
	bn_new((A)->y);															\
	bn_new((A)->p);															\
	bn_new((A)->q);															\
	(A)->t = 0;																\

#elif ALLOC == STACK
#define bdpe_new(A)															\
	A = (bdpe_t)alloca(sizeof(bdpe_st));									\
	bn_new((A)->n);															\
	bn_new((A)->y);															\
	bn_new((A)->p);															\
	bn_new((A)->q);															\

#endif

/**
 * Calls a function to clean and free a Benaloh's key pair.
 *
 * @param[out] A			- the key pair to clean and free.
 */
#if ALLOC == DYNAMIC
#define bdpe_free(A)														\
	if (A != NULL) {														\
		bn_free((A)->n);													\
		bn_free((A)->y);													\
		bn_free((A)->p);													\
		bn_free((A)->q);													\
		(A)->t = 0;															\
		free(A);															\
		A = NULL;															\
	}

#elif ALLOC == STATIC
#define bdpe_free(A)														\
	if (A != NULL) {														\
		bn_free((A)->n);													\
		bn_free((A)->y);													\
		bn_free((A)->p);													\
		bn_free((A)->q);													\
		(A)->t = 0;															\
		A = NULL;															\
	}																		\

#elif ALLOC == AUTO
#define bdpe_free(A)			/* empty */

#elif ALLOC == STACK
#define bdpe_free(A)														\
	bn_free((A)->n);														\
	bn_free((A)->y);														\
	bn_free((A)->p);														\
	bn_free((A)->q);														\
	(A)->t = 0;																\
	A = NULL;																\

#endif

/**
 * Initializes a SOKAKA key pair with a null value.
 *
 * @param[out] A			- the key pair to initialize.
 */
#if ALLOC == AUTO
#define sokaka_null(A)			/* empty */
#else
#define sokaka_null(A)		A = NULL;
#endif

/**
 * Calls a function to allocate and initialize a SOKAKA key pair.
 *
 * @param[out] A			- the new key pair.
 */
#if ALLOC == DYNAMIC
#define sokaka_new(A)														\
	A = (sokaka_t)calloc(1, sizeof(sokaka_st));								\
	if (A == NULL) {														\
		THROW(ERR_NO_MEMORY);												\
	}																		\
	g1_new((A)->s1);														\
	g2_new((A)->s2);														\

#elif ALLOC == STATIC
#define sokaka_new(A)														\
	A = (sokaka_t)alloca(sizeof(sokaka_st));								\
	if (A == NULL) {														\
		THROW(ERR_NO_MEMORY);												\
	}																		\
	g1_new((A)->s1);														\
	g2_new((A)->s2);														\

#elif ALLOC == AUTO
#define sokaka_new(A)			/* empty */

#elif ALLOC == STACK
#define sokaka_new(A)														\
	A = (sokaka_t)alloca(sizeof(sokaka_st));								\
	g1_new((A)->s1);														\
	g2_new((A)->s2);														\

#endif

/**
 * Calls a function to clean and free a SOKAKA key pair.
 *
 * @param[out] A			- the key pair to clean and free.
 */
#if ALLOC == DYNAMIC
#define sokaka_free(A)														\
	if (A != NULL) {														\
		g1_free((A)->s1);													\
		g2_free((A)->s2);													\
		free(A);															\
		A = NULL;															\
	}

#elif ALLOC == STATIC
#define sokaka_free(A)														\
	if (A != NULL) {														\
		g1_free((A)->s1);													\
		g2_free((A)->s2);													\
		A = NULL;															\
	}																		\

#elif ALLOC == AUTO
#define sokaka_free(A)			/* empty */

#elif ALLOC == STACK
#define sokaka_free(A)														\
	g1_free((A)->s1);														\
	g2_free((A)->s2);														\
	A = NULL;																\

#endif

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
 * @param[in] K				- the private key.
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
#if CP_RSA == BASIC
#define cp_rsa_dec(O, OL, I, IL, K)			cp_rsa_dec_basic(O, OL, I, IL, K)
#elif CP_RSA == QUICK
#define cp_rsa_dec(O, OL, I, IL, K)			cp_rsa_dec_quick(O, OL, I, IL, K)
#endif

/**
 * Signs a message using the RSA cryptosystem.
 *
 * @param[out] O			- the output buffer.
 * @param[out] OL			- the number of bytes written in the output buffer.
 * @param[in] I				- the input buffer.
 * @param[in] IL			- the number of bytes to sign.
 * @param[in] H				- the flag to indicate the message format.
 * @param[in] K				- the private key.
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
#if CP_RSA == BASIC
#define cp_rsa_sig(O, OL, I, IL, H, K)	cp_rsa_sig_basic(O, OL, I, IL, H, K)
#elif CP_RSA == QUICK
#define cp_rsa_sig(O, OL, I, IL, H, K)	cp_rsa_sig_quick(O, OL, I, IL, H, K)
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
 * @param[in, out] out_len	- the buffer capacity and number of bytes written.
 * @param[in] in			- the input buffer.
 * @param[in] in_len		- the number of bytes to encrypt.
 * @param[in] pub			- the public key.
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
int cp_rsa_enc(uint8_t *out, int *out_len, uint8_t *in, int in_len, rsa_t pub);

/**
 * Decrypts using the basic RSA decryption method.
 *
 * @param[out] out			- the output buffer.
 * @param[in, out] out_len	- the buffer capacity and number of bytes written.
 * @param[in] in			- the input buffer.
 * @param[in] in_len		- the number of bytes to decrypt.
 * @param[in] prv			- the private key.
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
int cp_rsa_dec_basic(uint8_t *out, int *out_len, uint8_t *in, int in_len,
		rsa_t prv);

/**
 * Decrypts using the fast RSA decryption with CRT optimization.
 *
 * @param[out] out			- the output buffer.
 * @param[in, out] out_len	- the buffer capacity and number of bytes written.
 * @param[in] in			- the input buffer.
 * @param[in] in_len		- the number of bytes to decrypt.
 * @param[in] prv			- the private key.
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
int cp_rsa_dec_quick(uint8_t *out, int *out_len, uint8_t *in, int in_len,
		rsa_t prv);

/**
 * Signs using the basic RSA signature algorithm. The flag must be non-zero if
 * the message being signed is already a hash value.
 *
 * @param[out] sig			- the signature
 * @param[out] sig_len		- the number of bytes written in the signature.
 * @param[in] msg			- the message to sign.
 * @param[in] msg_len		- the number of bytes to sign.
 * @param[in] hash			- the flag to indicate the message format.
 * @param[in] prv			- the private key.
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
int cp_rsa_sig_basic(uint8_t *sig, int *sig_len, uint8_t *msg, int msg_len,
		int hash, rsa_t prv);

/**
 * Signs using the fast RSA signature algorithm with CRT optimization. The flag
 * must be non-zero if the message being signed is already a hash value.
 *
 * @param[out] sig			- the signature
 * @param[out] sig_len		- the number of bytes written in the signature.
 * @param[in] msg			- the message to sign.
 * @param[in] msg_len		- the number of bytes to sign.
 * @param[in] hash			- the flag to indicate the message format.
 * @param[in] prv			- the private key.
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
int cp_rsa_sig_quick(uint8_t *sig, int *sig_len, uint8_t *msg, int msg_len,
		int hash, rsa_t prv);

/**
 * Verifies an RSA signature. The flag must be non-zero if the message being
 * signed is already a hash value.
 *
 * @param[in] sig			- the signature to verify.
 * @param[in] sig_len		- the signature length in bytes.
 * @param[in] msg			- the signed message.
 * @param[in] msg_len		- the message length in bytes.
 * @param[in] hash			- the flag to indicate the message format.
 * @param[in] pub			- the public key.
 * @return 1 if the signature is valid, 0 otherwise.
 */
int cp_rsa_ver(uint8_t *sig, int sig_len, uint8_t *msg, int msg_len, int hash,
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
 * @param[in, out] out_len	- the buffer capacity and number of bytes written.
 * @param[in] in			- the input buffer.
 * @param[in] in_len		- the number of bytes to encrypt.
 * @param[in] pub			- the public key.
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
int cp_rabin_enc(uint8_t *out, int *out_len, uint8_t *in, int in_len,
		rabin_t pub);

/**
 * Decrypts using the Rabin cryptosystem.
 *
 * @param[out] out			- the output buffer.
 * @param[in, out] out_len	- the buffer capacity and number of bytes written.
 * @param[in] in			- the input buffer.
 * @param[in] in_len		- the number of bytes to decrypt.
 * @param[in] prv			- the private key.
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
int cp_rabin_dec(uint8_t *out, int *out_len, uint8_t *in, int in_len,
		rabin_t prv);

/**
 * Generates a new key pair for Benaloh's Dense Probabilistic Encryption.
 *
 * @param[out] pub			- the public key.
 * @param[out] prv			- the private key.
 * @param[in] block			- the block size.
 * @param[in] bits			- the key length in bits.
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
int cp_bdpe_gen(bdpe_t pub, bdpe_t prv, dig_t block, int bits);

/**
 * Encrypts using Benaloh's cryptosystem.
 *
 * @param[out] out			- the output buffer.
 * @param[in, out] out_len	- the buffer capacity and number of bytes written.
 * @param[in] in			- the plaintext as a small integer.
 * @param[in] pub			- the public key.
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
int cp_bdpe_enc(uint8_t *out, int *out_len, dig_t in, bdpe_t pub);

/**
 * Decrypts using Benaloh's cryptosystem.
 *
 * @param[out] out			- the decrypted small integer.
 * @param[in] in			- the input buffer.
 * @param[in] in_len		- the number of bytes to encrypt.
 * @param[in] prv			- the private key.
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
int cp_bdpe_dec(dig_t *out, uint8_t *in, int in_len, bdpe_t prv);

/**
 * Generates a new key pair for Paillier's Homomorphic Probabilistic Encryption.
 *
 * @param[out] n			- the public key.
 * @param[out] l			- the private key.
 * @param[in] bits			- the key length in bits.
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
int cp_phpe_gen(bn_t n, bn_t l, int bits);

/**
 * Encrypts using the Paillier cryptosystem.
 *
 * @param[out] out			- the output buffer.
 * @param[in, out] out_len	- the buffer capacity and number of bytes written.
 * @param[in] in			- the input buffer.
 * @param[in] in_len		- the number of bytes to encrypt.
 * @param[in] n				- the public key.
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
int cp_phpe_enc(uint8_t *out, int *out_len, uint8_t *in, int in_len, bn_t n);

/**
 * Decrypts using the Paillier cryptosystem. Since this system is homomorphic,
 * no padding can be applied and the user is responsible for specifying the
 * resulting plaintext size.
 *
 * @param[out] out			- the output buffer.
 * @param[out] out_len		- the number of bytes to write in the output buffer.
 * @param[in] in_len		- the number of bytes to decrypt.
 * @param[in] n				- the public key.
 * @param[in] l				- the private key. 
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
int cp_phpe_dec(uint8_t *out, int out_len, uint8_t *in, int in_len, bn_t n,
		bn_t l);

/**
 * Generates an ECDH key pair.
 *
 * @param[out] d			- the private key.
 * @param[in] q				- the public key.
 */
int cp_ecdh_gen(bn_t d, ec_t q);

/**
 * Derives a shared secret using ECDH.
 *
 * @param[out] key				- the shared key.
 * @param[int] key_len			- the intended shared key length in bytes.
 * @param[in] d					- the private key.
 * @param[in] q					- the point received from the other party.
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
int cp_ecdh_key(uint8_t *key, int key_len, bn_t d, ec_t q);

/**
 * Generate an ECMQV key pair.
 *
 * Should also be used to generate the ephemeral key pair.
 *
 * @param[out] d			- the private key.
 * @param[in] q				- the public key.
 */
int cp_ecmqv_gen(bn_t d, ec_t q);

/**
 * Derives a shared secret using ECMQV.
 *
 * @param[out] key				- the shared key.
 * @param[int] key_len			- the intended shared key length in bytes.
 * @param[in] d1				- the private key.
 * @param[in] d2				- the ephemeral private key.
 * @param[in] q2u				- the ephemeral public key.
 * @param[in] q1v				- the point received from the other party.
 * @param[in] q2v				- the ephemeral point received from the other party.
 */
int cp_ecmqv_key(uint8_t *key, int key_len, bn_t d1, bn_t d2, ec_t q2u,
		ec_t q1v, ec_t q2v);

/**
 * Generates an ECIES key pair.
 *
 * @param[out] d			- the private key.
 * @param[in] q				- the public key.
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
int cp_ecies_gen(bn_t d, ec_t q);

/**
 * Encrypts using the ECIES cryptosystem.
 *
 * @param[out] r 			- the resulting elliptic curve point.
 * @param[out] out			- the output buffer.
 * @param[in, out] out_len	- the buffer capacity and number of bytes written.
 * @param[in] in			- the input buffer. 
 * @param[in] in_len		- the number of bytes to encrypt.
 * @param[in] iv 			- the block cipher initialization vector.
 * @param[in] q				- the public key.
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
int cp_ecies_enc(ec_t r, uint8_t *out, int *out_len, uint8_t *in, int in_len,
		ec_t q);

/**
 * Decrypts using the ECIES cryptosystem.
 *
 * @param[out] out			- the output buffer.
 * @param[in, out] out_len	- the buffer capacity and number of bytes written.
 * @param[in] in			- the input buffer.
 * @param[in] in_len		- the number of bytes to encrypt.
 * @param[in] iv 			- the block cipher initialization vector.
 * @param[in] d				- the private key.
 * @return STS_OK if no errors occurred, STS_ERR otherwise.
 */
int cp_ecies_dec(uint8_t *out, int *out_len, ec_t r, uint8_t *in, int in_len,
		bn_t d);

/**
 * Generates an ECDSA key pair.
 *
 * @param[out] d			- the private key.
 * @param[in] q				- the public key.
 */
int cp_ecdsa_gen(bn_t d, ec_t q);

/**
 * Signs a message using ECDSA.
 *
 * @param[out] r				- the first component of the signature.
 * @param[out] s				- the second component of the signature.
 * @param[in] msg				- the message to sign.
 * @param[in] len				- the message length in bytes.
 * @param[in] hash				- the flag to indicate the message format.
 * @param[in] d					- the private key.
 */
int cp_ecdsa_sig(bn_t r, bn_t s, uint8_t *msg, int len, int hash, bn_t d);

/**
 * Verifies a message signed with ECDSA using the basic method.
 *
 * @param[out] r				- the first component of the signature.
 * @param[out] s				- the second component of the signature.
 * @param[in] msg				- the message to sign.
 * @param[in] len				- the message length in bytes.
 * @param[in] hash				- the flag to indicate the message format.
 * @param[in] q					- the public key.
 */
int cp_ecdsa_ver(bn_t r, bn_t s, uint8_t *msg, int len, int hash, ec_t q);

/**
 * Generates an Elliptic Curve Schnorr Signature key pair.
 *
 * @param[out] d			- the private key.
 * @param[in] q				- the public key.
 */
int cp_ecss_gen(bn_t d, ec_t q);

/**
 * Signs a message using the Elliptic Curve Schnorr Signature.
 *
 * @param[out] r				- the first component of the signature.
 * @param[out] s				- the second component of the signature.
 * @param[in] msg				- the message to sign.
 * @param[in] len				- the message length in bytes.
 * @param[in] d					- the private key.
 */
int cp_ecss_sig(bn_t e, bn_t s, uint8_t *msg, int len, bn_t d);

/**
 * Verifies a message signed with the Elliptic Curve Schnorr Signature using the
 * basic method.
 *
 * @param[out] r				- the first component of the signature.
 * @param[out] s				- the second component of the signature.
 * @param[in] msg				- the message to sign.
 * @param[in] len				- the message length in bytes.
 * @param[in] q					- the public key.
 */
int cp_ecss_ver(bn_t e, bn_t s, uint8_t *msg, int len, ec_t q);

/**
 * Generates a master key for the SOK identity-based non-interactive
 * authenticated key agreement protocol.
 *
 * @param[out] master			- the master key.
 */
int cp_sokaka_gen(bn_t master);

/**
 * Generates a private key for the SOK protocol.
 *
 * @param[out] k				- the private key.
 * @param[in] id				- the identity.
 * @param[in] len				- the length of identity in bytes.
 * @param[in] master			- the master key.
 */
int cp_sokaka_gen_prv(sokaka_t k, char *id, int len, bn_t master);

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
int cp_sokaka_key(uint8_t *key, unsigned int key_len, char *id1, int len1,
		sokaka_t k, char *id2, int len2);

/**
 * Generates a BLS key pair.
 *
 * @param[out] d			- the private key.
 * @param[in] q				- the public key.
 */
int cp_bls_gen(bn_t d, g2_t q);

/**
 * Signs a message using BLS.
 *
 * @param[out] s				- the signature.
 * @param[in] msg				- the message to sign.
 * @param[in] len				- the message length in bytes.
 * @param[in] d					- the private key.
 */
int cp_bls_sig(g1_t s, uint8_t *msg, int len, bn_t d);

/**
 * Verifies a message signed with BLS using the basic method.
 *
 * @param[out] s				- the signature.
 * @param[in] msg				- the message to sign.
 * @param[in] len				- the message length in bytes.
 * @param[in] q					- the public key.
 * @return a boolean value indicating the verification result.
 */
int cp_bls_ver(g1_t s, uint8_t *msg, int len, g2_t q);

/**
 * Generates a Boneh-Boyen key pair.
 *
 * @param[out] d			- the private key.
 * @param[in] q				- the first component of the public key.
 * @param[in] q				- the second component of the public key.
 */
int cp_bbs_gen(bn_t d, g2_t q, gt_t z);

/**
 * Signs a message using Boneh-Boyen Short Signature.
 *
 * @param[out] s				- the signature.
 * @param[in] msg				- the message to sign.
 * @param[in] len				- the message length in bytes.
 * @param[in] hash				- the flag to indicate the message format.
 * @param[in] d					- the private key.
 */
int cp_bbs_sig(g1_t s, uint8_t *msg, int len, int hash, bn_t d);

/**
 * Verifies a message signed with BLS using the basic method.
 *
 * @param[in] s					- the signature.
 * @param[in] msg				- the message to sign.
 * @param[in] len				- the message length in bytes.
 * @param[in] hash				- the flag to indicate the message format.
 * @param[in] q					- the public key.
 * @return a boolean value indicating the verification result.
 */
int cp_bbs_ver(g1_t s, uint8_t *msg, int len, int hash, g2_t q, gt_t z);

#endif /* !RELIC_CP_H */
