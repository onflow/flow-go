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
 * @file
 *
 * Implementation of the RSA cryptosystem.
 *
 * @version $Id$
 * @ingroup cp
 */

#include <string.h>

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_error.h"
#include "relic_rand.h"
#include "relic_bn.h"
#include "relic_util.h"
#include "relic_cp.h"
#include "relic_md.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

/**
 * Default RSA public exponent.
 */
#define RSA_EXP 			"65537"

/**
 * Length of chosen padding scheme.
 */
#if CP_RSAPD == PKCS1
#define RSA_PAD_LEN		(11)
#elif CP_RSAPD == PKCS2
#define RSA_PAD_LEN		(2 * MD_LEN + 2)
#else
#define RSA_PAD_LEN		(0)
#endif

/**
 * Identifier for encrypted messages.
 */
#define RSA_TYPE_PUB		(02)

/**
 * Identifier for signed messages.
 */
#define RSA_TYPE_PRV		(01)

/**
 * Byte used as padding unit in signatures.
 */
#define RSA_PAD_PRV			(0xFF)

/**
 * Identifier for encryption.
 */
#define RSA_ENC				1

/**
 * Identifier for decryption.
 */
#define RSA_DEC				2

/**
 * Identifier for encryption.
 */
#define RSA_SIGN			3

/**
 * Identifier for decryption.
 */
#define RSA_VER				4

/**
 * Applies or removes a PKCS#1 v1.5 encryption padding.
 *
 * @param[out] m		- the buffer to pad.
 * @param[out] p_len	- the number of added pad bytes.
 * @param[in] m_len		- the message length in bytes.
 * @param[in] k_len		- the key length in bytes.
 * @param[in] operation	- flag to indicate the operation type.
 * @return STS_ERR if errors occurred, STS_OK otherwise.
 */
static int pad_pkcs1(bn_t m, int *p_len, int m_len, int k_len, int operation) {
	unsigned char pad = 0;
	int result = STS_OK;
	bn_t t;

	bn_null(t);
	bn_new(t);

	switch (operation) {
			/* EB = 00 | 02 | PS | 00 | D. */
		case RSA_ENC:
			bn_zero(m);
			bn_lsh(m, m, 8);
			bn_add_dig(m, m, RSA_TYPE_PUB);

			*p_len = k_len - 3 - m_len;
			for (int i = 0; i < *p_len; i++) {
				bn_lsh(m, m, 8);
				do {
					rand_bytes(&pad, 1);
				} while (pad == 0);
				bn_add_dig(m, m, pad);
			}
			bn_lsh(m, m, 8);
			bn_add_dig(m, m, 0);
			/* Make room for the real message. */
			bn_lsh(m, m, m_len * 8);
			break;
		case RSA_DEC:
			m_len = k_len - 1;
			bn_rsh(t, m, 8 * m_len);
			if (!bn_is_zero(t)) {
				result = STS_ERR;
			} else {
				*p_len = m_len;
				m_len--;
				bn_rsh(t, m, 8 * m_len);
				pad = (unsigned char)t->dp[0];
				if (pad != RSA_TYPE_PUB) {
					result = STS_ERR;
				} else {
					m_len--;
					bn_rsh(t, m, 8 * m_len);
					pad = (unsigned char)t->dp[0];
					while (pad != 0) {
						m_len--;
						bn_rsh(t, m, 8 * m_len);
						pad = (unsigned char)t->dp[0];
					}
					/* Remove padding and trailing zero. */
					*p_len -= (m_len - 1);
					bn_mod_2b(m, m, (k_len - *p_len) * 8);
				}
			}
			break;
			/* EB = 00 | 01 | PS | 00 | D. */
		case RSA_SIGN:
			bn_zero(m);
			bn_lsh(m, m, 8);
			bn_add_dig(m, m, RSA_TYPE_PRV);

			*p_len = k_len - 3 - m_len;
			for (int i = 0; i < *p_len; i++) {
				bn_lsh(m, m, 8);
				bn_add_dig(m, m, RSA_PAD_PRV);
			}
			bn_lsh(m, m, 8);
			bn_add_dig(m, m, 0);
			/* Make room for the real message. */
			bn_lsh(m, m, m_len * 8);
			break;
		case RSA_VER:
			m_len = k_len - 1;
			bn_rsh(t, m, 8 * m_len);
			if (!bn_is_zero(t)) {
				result = STS_ERR;
			} else {
				*p_len = m_len;
				m_len--;
				bn_rsh(t, m, 8 * m_len);
				pad = (unsigned char)t->dp[0];
				if (pad != RSA_TYPE_PRV) {
					result = STS_ERR;
				} else {
					m_len--;
					bn_rsh(t, m, 8 * m_len);
					pad = (unsigned char)t->dp[0];
					while (pad == RSA_PAD_PRV) {
						m_len--;
						bn_rsh(t, m, 8 * m_len);
						pad = (unsigned char)t->dp[0];
					}
					/* Remove padding and trailing zero. */
					*p_len -= (m_len);
					bn_mod_2b(m, m, (k_len - *p_len) * 8);
				}
			}
			break;
	}

	bn_free(t);
	return result;
}

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#if CP_RSA == BASIC || !defined(STRIP)

int cp_rsa_gen_basic(rsa_t pub, rsa_t prv, int bits) {
	bn_t t, r;
	int result = STS_OK;

	bn_null(t);
	bn_null(r);

	TRY {
		bn_new(t);
		bn_new(r);

		/* Generate different primes p and q. */
		do {
			bn_gen_prime(prv->p, bits / 2);
			bn_gen_prime(prv->q, bits / 2);
		} while (bn_cmp(prv->p, prv->q) == CMP_EQ);

		/* Swap p and q so that p is smaller. */
		if (bn_cmp(prv->p, prv->q) == CMP_LT) {
			bn_copy(t, prv->p);
			bn_copy(prv->p, prv->q);
			bn_copy(prv->q, t);
		}

		bn_mul(pub->n, prv->p, prv->q);
		bn_copy(prv->n, pub->n);
		bn_sub_dig(prv->p, prv->p, 1);
		bn_sub_dig(prv->q, prv->q, 1);

		bn_mul(t, prv->p, prv->q);

		bn_read_str(pub->e, RSA_EXP, strlen(RSA_EXP), 10);

		bn_gcd_ext(r, prv->d, NULL, pub->e, t);
		if (bn_sign(prv->d) == BN_NEG) {
			bn_add(prv->d, prv->d, t);
		}

		if (bn_cmp_dig(r, 1) == CMP_EQ) {
			bn_add_dig(prv->p, prv->p, 1);
			bn_add_dig(prv->q, prv->q, 1);
		}
	}
	CATCH_ANY {
		result = STS_ERR;
	}
	FINALLY {
		bn_free(t);
		bn_free(r);
	}

	return result;
}

#endif

#if CP_RSA == QUICK || !defined(STRIP)

int cp_rsa_gen_quick(rsa_t pub, rsa_t prv, int bits) {
	bn_t t, r;
	int result = STS_OK;

	bn_null(t);
	bn_null(r);

	TRY {
		bn_new(t);
		bn_new(r);

		/* Generate different primes p and q. */
		do {
			bn_gen_prime(prv->p, bits / 2);
			bn_gen_prime(prv->q, bits / 2);
		} while (bn_cmp(prv->p, prv->q) == CMP_EQ);

		/* Swap p and q so that p is smaller. */
		if (bn_cmp(prv->p, prv->q) == CMP_LT) {
			bn_copy(t, prv->p);
			bn_copy(prv->p, prv->q);
			bn_copy(prv->q, t);
		}

		/* n = pq. */
		bn_mul(pub->n, prv->p, prv->q);
		bn_copy(prv->n, pub->n);
		bn_sub_dig(prv->p, prv->p, 1);
		bn_sub_dig(prv->q, prv->q, 1);

		/* phi(n) = (p - 1)(q - 1). */
		bn_mul(t, prv->p, prv->q);

		bn_read_str(pub->e, RSA_EXP, strlen(RSA_EXP), 10);

		/* d = e^(-1) mod phi(n). */
		bn_gcd_ext(r, prv->d, NULL, pub->e, t);
		if (bn_sign(prv->d) == BN_NEG) {
			bn_add(prv->d, prv->d, t);
		}

		if (bn_cmp_dig(r, 1) == CMP_EQ) {
			/* dP = d mod (p - 1). */
			bn_mod(prv->dp, prv->d, prv->p);
			/* dQ = d mod (q - 1). */
			bn_mod(prv->dq, prv->d, prv->q);

			bn_add_dig(prv->p, prv->p, 1);
			bn_add_dig(prv->q, prv->q, 1);

			/* qInv = q^(-1) mod p. */
			bn_gcd_ext(r, prv->qi, NULL, prv->q, prv->p);
			if (bn_sign(prv->qi) == BN_NEG) {
				bn_add(prv->qi, prv->qi, prv->p);
			}

			result = STS_OK;
		}
	}
	CATCH_ANY {
		result = STS_ERR;
	}
	FINALLY {
		bn_free(t);
		bn_free(r);
	}

	return result;
}

#endif

int cp_rsa_enc(unsigned char *out, int *out_len, unsigned char *in, int in_len,
		rsa_t pub) {
	bn_t m, eb;
	int sign, size, pad_len, result = STS_OK;

	bn_null(m);
	bn_null(eb);

	bn_size_bin(&size, pub->n);

	if (in_len > (size - RSA_PAD_LEN)) {
		return STS_ERR;
	}

	TRY {
		bn_new(m);
		bn_zero(m);
		bn_new(eb);
		bn_zero(eb);

#if CP_RSAPD == PKCS1
		if (pad_pkcs1(eb, &pad_len, in_len, size, RSA_ENC) == STS_OK) {
#elif CP_RSAPD == PKCS2
		if (pad_pkcs2(eb, &pad_len, in_len, size, RSA_ENC) == STS_OK) {
#else
		pad_len = 0;
		if (1) {
#endif
			bn_read_bin(m, in, in_len, BN_POS);
			bn_add(eb, eb, m);

			bn_mxp(eb, eb, pub->e, pub->n);

			if (size <= *out_len) {
				*out_len = size;
				memset(out, 0, *out_len);
				bn_write_bin(out, &size, &sign, eb);
			} else {
				result = STS_ERR;
			}

			if (sign == BN_NEG) {
				result = STS_ERR;
			}
		}
	}
	CATCH_ANY {
		result = STS_ERR;
	}
	FINALLY {
		bn_free(m);
		bn_free(eb);
	}

	return result;
}

#if CP_RSA == BASIC || !defined(STRIP)

int cp_rsa_dec_basic(unsigned char *out, int *out_len, unsigned char *in,
		int in_len, rsa_t prv) {
	bn_t m, eb;
	int sign, size, pad_len, result = STS_OK;

	bn_size_bin(&size, prv->n);

	if (in_len < 0 || in_len != size || in_len < RSA_PAD_LEN) {
		return STS_ERR;
	}

	bn_null(m);
	bn_null(eb);

	TRY {
		bn_new(m);
		bn_new(eb);

		bn_read_bin(eb, in, in_len, BN_POS);
		bn_mxp(eb, eb, prv->d, prv->n);

#if CP_RSAPD == PKCS1
		if (pad_pkcs1(eb, &pad_len, in_len, size, RSA_DEC) == STS_OK) {
#elif CP_RSAPD == PKCS2
		if (pad_pkcs2(eb, &pad_len, in_len, size, RSA_DEC) == STS_OK) {
#else
		pad_len = 0;
		if (1) {
#endif
			size = size - pad_len;

			if (size <= *out_len) {
				memset(out, 0, size);
				bn_size_bin(&size, eb);
				bn_write_bin(out, &size, &sign, eb);
				*out_len = size;
			} else {
				result = STS_ERR;
			}

			if (sign == BN_NEG) {
				result = STS_ERR;
			}
		}
	}
	CATCH_ANY {
		result = STS_ERR;
	}
	FINALLY {
		bn_free(m);
		bn_free(eb);
	}

	return result;
}

#endif

#if CP_RSA == QUICK || !defined(STRIP)

int cp_rsa_dec_quick(unsigned char *out, int *out_len, unsigned char *in,
		int in_len, rsa_t prv) {
	bn_t m, eb;
	int sign, size, pad_len, result = STS_OK;

	bn_null(m);
	bn_null(eb);

	bn_size_bin(&size, prv->n);

	if (in_len < 0 || in_len > size) {
		return STS_ERR;
	}

	TRY {
		bn_new(m);
		bn_new(eb);

		bn_read_bin(eb, in, in_len, BN_POS);

		bn_copy(m, eb);

		/* m1 = c^dP mod p. */
		bn_mxp(eb, eb, prv->dp, prv->p);

		/* m2 = c^dQ mod q. */
		bn_mxp(m, m, prv->dq, prv->q);

		/* m1 = m1 - m2 mod p. */
		bn_sub(eb, eb, m);
		while (bn_sign(eb) == BN_NEG) {
			bn_add(eb, eb, prv->p);
		}
		bn_mod(eb, eb, prv->p);
		/* m1 = qInv(m1 - m2) mod p. */
		bn_mul(eb, eb, prv->qi);
		bn_mod(eb, eb, prv->p);
		/* m = m2 + m1 * q. */
		bn_mul(eb, eb, prv->q);
		bn_add(eb, eb, m);
		bn_mod(eb, eb, prv->n);

#if CP_RSAPD == PKCS1
		if (pad_pkcs1(eb, &pad_len, in_len, size, RSA_DEC) == STS_OK) {
#elif CP_RSAPD == PKCS2
		if (pad_pkcs2(eb, &pad_len, in_len, size, RSA_DEC) == STS_OK) {
#else
		pad_len = 0;
		if (1) {
#endif
			size = size - pad_len;

			if (size <= *out_len) {
				memset(out, 0, size);
				bn_size_bin(&size, eb);
				bn_write_bin(out, &size, &sign, eb);
				*out_len = size;
			} else {
				result = STS_ERR;
			}

			if (sign == BN_NEG) {
				result = STS_ERR;
			}
		}
	}
	CATCH_ANY {
		result = STS_ERR;
	}
	FINALLY {
		bn_free(m);
		bn_free(eb);
	}

	return result;
}

#endif

#if CP_RSA == BASIC || !defined(STRIP)

int cp_rsa_sign_basic(unsigned char *sig, int *sig_len, unsigned char *msg,
		int msg_len, rsa_t prv) {
	bn_t m, eb;
	int sign, size, pad_len, result = STS_OK;
	unsigned char hash[MD_LEN];

	bn_null(m);
	bn_null(eb);
	bn_size_bin(&size, prv->n);

	if (MD_LEN > (size - RSA_PAD_LEN)) {
		return STS_ERR;
	}

	TRY {
		bn_new(m);
		bn_new(eb);

		bn_zero(m);
		bn_zero(eb);

#if CP_RSAPD == PKCS1
		if (pad_pkcs1(eb, &pad_len, MD_LEN, size, RSA_SIGN) == STS_OK) {
#elif CP_RSAPD == PKCS2
		if (pad_pkcs2(eb, &pad_len, MD_LEN, size, RSA_SIGN) == STS_OK) {
#else
		pad_len = 0;
		if (1) {
#endif
			md_map(hash, msg, msg_len);
			bn_read_bin(m, hash, MD_LEN, BN_POS);
			bn_add(eb, eb, m);

			bn_mxp(eb, eb, prv->d, prv->n);

			bn_size_bin(&size, prv->n);

			if (size <= *sig_len) {
				*sig_len = size;
				memset(sig, 0, *sig_len);
				bn_write_bin(sig, &size, &sign, eb);
			} else {
				result = STS_ERR;
			}

			if (sign == BN_NEG) {
				result = STS_ERR;
			}
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(m);
		bn_free(eb);
	}

	return result;
}

#endif

#if CP_RSA == QUICK || !defined(STRIP)

int cp_rsa_sign_quick(unsigned char *sig, int *sig_len, unsigned char *msg,
		int msg_len, rsa_t prv) {
	bn_t m, eb;
	int sign, pad_len, size, result = STS_OK;
	unsigned char hash[MD_LEN];

	bn_size_bin(&size, prv->n);

	if (MD_LEN == size) {
		return STS_ERR;
	}
	if (MD_LEN > (size - RSA_PAD_LEN)) {
		return STS_ERR;
	}

	bn_null(m);
	bn_null(eb);

	TRY {
		bn_new(m);
		bn_new(eb);

		bn_zero(m);
		bn_zero(eb);

#if CP_RSAPD == PKCS1
		if (pad_pkcs1(eb, &pad_len, MD_LEN, size, RSA_SIGN) == STS_OK) {
#elif CP_RSAPD == PKCS2
		if (pad_pkcs2(eb, &pad_len, MD_LEN, size, RSA_SIGN) == STS_OK) {
#else
		pad_len = 0;
		if (1) {
#endif
			md_map(hash, msg, msg_len);
			bn_read_bin(m, hash, MD_LEN, BN_POS);
			bn_add(eb, eb, m);

			bn_copy(m, eb);

			/* m1 = c^dP mod p. */
			bn_mxp(eb, eb, prv->dp, prv->p);

			/* m2 = c^dQ mod q. */
			bn_mxp(m, m, prv->dq, prv->q);

			/* m1 = m1 - m2 mod p. */
			bn_sub(eb, eb, m);
			while (bn_sign(eb) == BN_NEG) {
				bn_add(eb, eb, prv->p);
			}
			bn_mod(eb, eb, prv->p);
			/* m1 = qInv(m1 - m2) mod p. */
			bn_mul(eb, eb, prv->qi);
			bn_mod(eb, eb, prv->p);
			/* m = m2 + m1 * q. */
			bn_mul(eb, eb, prv->q);
			bn_add(eb, eb, m);
			bn_mod(eb, eb, prv->n);

			bn_size_bin(&size, prv->n);

			if (size <= *sig_len) {
				*sig_len = size;
				memset(sig, 0, *sig_len);
				bn_write_bin(sig, &size, &sign, eb);
			} else {
				result = STS_ERR;
			}

			if (sign == BN_NEG) {
				result = STS_ERR;
			}
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(m);
		bn_free(eb);
	}

	return result;
}

#endif

int cp_rsa_ver(unsigned char *sig, int sig_len, unsigned char *msg,
		int msg_len, rsa_t pub) {
	bn_t m, eb;
	int sign, size, pad_len, result;
	unsigned char hash1[MD_LEN], hash2[MD_LEN];

	/* We suppose that the signature is invalid. */
	result = 0;
	bn_size_bin(&size, pub->n);

	bn_null(m);
	bn_null(eb);

	TRY {
		bn_new(m);
		bn_new(eb);

		bn_read_bin(eb, sig, sig_len, BN_POS);

		bn_mxp(eb, eb, pub->e, pub->n);

#if CP_RSAPD == PKCS1
		if (pad_pkcs1(eb, &pad_len, MD_LEN, size, RSA_VER) == STS_OK) {
#elif CP_RSAPD == PKCS2
		if (pad_pkcs2(eb, &pad_len, MD_LEN, size, RSA_VER) == STS_OK) {
#else
		pad_len = 0;
		if (1) {
#endif
			memset(hash1, 0, MD_LEN);
			bn_write_bin(hash1, &size, &sign, eb);

			if (sign == BN_POS) {
				md_map(hash2, msg, msg_len);
				if (memcmp(hash1, hash2, MD_LEN) == 0) {
					/* Everything went ok, so signature status is changed. */
					result = 1;
				}
			}
		}
	}
	CATCH_ANY {
		result = 0;
	}
	FINALLY {
		bn_free(m);
		bn_free(eb);
	}

	return result;
}
