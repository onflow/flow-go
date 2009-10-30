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
 * Implementation of the prime number generation and testing functions.
 *
 * Strong prime generation is based on Gordon's Algorithm, taken from Handbook
 * of Applied Cryptography.
 *
 * @version $Id$
 * @ingroup bn
 */

#include "relic_core.h"
#include "relic_bn.h"
#include "relic_bn_low.h"
#include "relic_util.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

/**
 * Number of trial division tests.
 */
#define BASIC_TESTS	((int)(sizeof(primes)/(sizeof(dig_t))))

/**
 * Small prime numbers table.
 */
static const dig_t primes[] = {
	0x0002, 0x0003, 0x0005, 0x0007, 0x000B, 0x000D, 0x0011, 0x0013,
	0x0017, 0x001D, 0x001F, 0x0025, 0x0029, 0x002B, 0x002F, 0x0035,
	0x003B, 0x003D, 0x0043, 0x0047, 0x0049, 0x004F, 0x0053, 0x0059,
	0x0061, 0x0065, 0x0067, 0x006B, 0x006D, 0x0071, 0x007F, 0x0083,
	0x0089, 0x008B, 0x0095, 0x0097, 0x009D, 0x00A3, 0x00A7, 0x00AD,
	0x00B3, 0x00B5, 0x00BF, 0x00C1, 0x00C5, 0x00C7, 0x00D3, 0x00DF,
#if WORD > 8
	0x00E3, 0x00E5, 0x00E9, 0x00EF, 0x00F1, 0x00FB, 0x0101, 0x0107,
	0x010D, 0x010F, 0x0115, 0x0119, 0x011B, 0x0125, 0x0133, 0x0137,

	0x0139, 0x013D, 0x014B, 0x0151, 0x015B, 0x015D, 0x0161, 0x0167,
	0x016F, 0x0175, 0x017B, 0x017F, 0x0185, 0x018D, 0x0191, 0x0199,
	0x01A3, 0x01A5, 0x01AF, 0x01B1, 0x01B7, 0x01BB, 0x01C1, 0x01C9,
	0x01CD, 0x01CF, 0x01D3, 0x01DF, 0x01E7, 0x01EB, 0x01F3, 0x01F7,
	0x01FD, 0x0209, 0x020B, 0x021D, 0x0223, 0x022D, 0x0233, 0x0239,
	0x023B, 0x0241, 0x024B, 0x0251, 0x0257, 0x0259, 0x025F, 0x0265,
	0x0269, 0x026B, 0x0277, 0x0281, 0x0283, 0x0287, 0x028D, 0x0293,
	0x0295, 0x02A1, 0x02A5, 0x02AB, 0x02B3, 0x02BD, 0x02C5, 0x02CF,

	0x02D7, 0x02DD, 0x02E3, 0x02E7, 0x02EF, 0x02F5, 0x02F9, 0x0301,
	0x0305, 0x0313, 0x031D, 0x0329, 0x032B, 0x0335, 0x0337, 0x033B,
	0x033D, 0x0347, 0x0355, 0x0359, 0x035B, 0x035F, 0x036D, 0x0371,
	0x0373, 0x0377, 0x038B, 0x038F, 0x0397, 0x03A1, 0x03A9, 0x03AD,
	0x03B3, 0x03B9, 0x03C7, 0x03CB, 0x03D1, 0x03D7, 0x03DF, 0x03E5,
	0x03F1, 0x03F5, 0x03FB, 0x03FD, 0x0407, 0x0409, 0x040F, 0x0419,
	0x041B, 0x0425, 0x0427, 0x042D, 0x043F, 0x0443, 0x0445, 0x0449,
	0x044F, 0x0455, 0x045D, 0x0463, 0x0469, 0x047F, 0x0481, 0x048B,

	0x0493, 0x049D, 0x04A3, 0x04A9, 0x04B1, 0x04BD, 0x04C1, 0x04C7,
	0x04CD, 0x04CF, 0x04D5, 0x04E1, 0x04EB, 0x04FD, 0x04FF, 0x0503,
	0x0509, 0x050B, 0x0511, 0x0515, 0x0517, 0x051B, 0x0527, 0x0529,
	0x052F, 0x0551, 0x0557, 0x055D, 0x0565, 0x0577, 0x0581, 0x058F,
	0x0593, 0x0595, 0x0599, 0x059F, 0x05A7, 0x05AB, 0x05AD, 0x05B3,
	0x05BF, 0x05C9, 0x05CB, 0x05CF, 0x05D1, 0x05D5, 0x05DB, 0x05E7,
	0x05F3, 0x05FB, 0x0607, 0x060D, 0x0611, 0x0617, 0x061F, 0x0623,
	0x062B, 0x062F, 0x063D, 0x0641, 0x0647, 0x0649, 0x064D, 0x0653
#endif
};

/**
 * Counts the number of lower significant bits of a multiple precision integer
 * which are zero.
 *
 * @param[in] a				- the multiple precision integer to examine.
 * @return the number of lower significant bits matching the criteria.
 */
static int bn_count(bn_t a) {
	static const int tab[16] =
			{ 4, 0, 1, 0, 2, 0, 1, 0, 3, 0, 1, 0, 2, 0, 1, 0 };
	int i;
	dig_t q, qq;

	if (bn_is_zero(a)) {
		return 0;
	}

	for (i = 0; i < a->used && a->dp[i] == 0; i++) ;
	q = a->dp[i];
	i *= BN_DIGIT;

	if ((q & 1) == 0) {
		do {
			qq = q & 15;
			i += tab[qq];
			q >>= 4;
		} while (qq == 0);
	}
	return i;
}

/**
 * Computes c = a & b mod m.
 *
 * @param c				- the result.
 * @param a				- the basis.
 * @param b				- the exponent.
 * @param m				- the modulus.
 */
static void bn_exp(bn_t c, bn_t a, bn_t b, bn_t m) {
	int i, l;
	bn_t t;

	bn_null(t);

	TRY {
		bn_new(t);

		l = bn_bits(b);

		bn_copy(t, a);

		for (i = l - 2; i >= 0; i--) {
			bn_sqr(t, t);
			bn_mod_basic(t, t, m);
			if (bn_test_bit(b, i)) {
				bn_mul(t, t, a);
				bn_mod_basic(t, t, m);
			}
		}

		bn_copy(c, t);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(t);
	}
}

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

int bn_is_prime(bn_t a) {
	int result;

	result = 0;
	if (!bn_is_prime_basic(a)) {
		goto end;
	}

	if (!bn_is_prime_rabin(a)) {
		goto end;
	}

	result = 1;
  end:
	return result;
}

int bn_is_prime_basic(bn_t a) {
	dig_t t;
	int i, result;

	result = 1;

	/* Trial division. */
	for (i = 0; i < BASIC_TESTS; i++) {
		bn_mod_dig(&t, a, primes[i]);
		if (t == 0) {
			result = 0;
			break;
		}
	}
	return result;
}

int bn_is_prime_rabin(bn_t a) {
	bn_t t, n1, y, r;
	int i, s, j, result, b, tests = 0;

	tests = 0;
	result = 1;

	bn_null(t);
	bn_null(n1);
	bn_null(y);
	bn_null(r);

	TRY {
		/*
		 * These values are taken from Table 4.4 inside Handbook of Applied
		 * Cryptography.
		 */
		b = bn_bits(a);
		if (b >= 1300) {
			tests = 2;
		} else if (b >= 850) {
			tests = 3;
		} else if (b >= 650) {
			tests = 4;
		} else if (b >= 550) {
			tests = 5;
		} else if (b >= 450) {
			tests = 6;
		} else if (b >= 400) {
			tests = 7;
		} else if (b >= 350) {
			tests = 8;
		} else if (b >= 300) {
			tests = 9;
		} else if (b >= 250) {
			tests = 12;
		} else if (b >= 200) {
			tests = 15;
		} else if (b >= 150) {
			tests = 18;
		} else {
			tests = 27;
		}

		bn_new(t);
		bn_new(n1);
		bn_new(y);
		bn_new(r);

		for (i = 0; i < tests; i++) {
			do {
				bn_rand(t, BN_POS, bn_bits(a));
				bn_mod_basic(t, t, a);
				bn_sub_dig(t, t, 1);
			} while (bn_cmp_dig(t, 1) != CMP_GT);

			bn_sub_dig(n1, a, 1);

			bn_copy(r, n1);

			s = bn_count(r);

			/* r = (n - 1)/2^s. */
			bn_rsh(r, r, s);

			/* y = b^r mod a. */
			bn_exp(y, t, r, a);

			if (bn_cmp_dig(y, 1) != CMP_EQ && bn_cmp(y, n1) != CMP_EQ) {
				j = 1;
				while ((j <= (s - 1)) && bn_cmp(y, n1) != CMP_EQ) {
					bn_sqr(y, y);
					bn_mod_basic(y, y, a);

					/* If y == 1 then composite. */
					if (bn_cmp_dig(y, 1) == CMP_EQ) {
						result = 0;
						break;
					}
					++j;
				}

				/* If y != n1 then composite. */
				if (bn_cmp(y, n1) != CMP_EQ) {
					result = 0;
					break;
				}
			}
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
		return 0;
	}
	FINALLY {
		bn_free(r);
		bn_free(y);
		bn_free(n1);
		bn_free(t);
	}
	return result;
}

int bn_is_prime_solov(bn_t a) {
	bn_t t0, t1, t2;
	int i, result;

	bn_null(t0);
	bn_null(t1);
	bn_null(t2);

	result = 1;

	TRY {
		bn_new(t0);
		bn_new(t1);
		bn_new(t2);

		for (i = 0; i < 100; i++) {
			/* Generate t0, 2 <= t0, <= a - 2. */
			do {
				bn_rand(t0, BN_POS, bn_bits(a));
				bn_mod_basic(t0, t0, a);
			} while (bn_cmp_dig(t0, 2) == CMP_LT);
			/* t2 = a - 1. */
			bn_copy(t2, a);
			bn_sub_dig(t2, t2, 1);
			/* t1 = (a - 1)/2. */
			bn_rsh(t1, t2, 1);
			/* t1 = t0^(a - 1)/2 mod a. */
			bn_exp(t1, t0, t1, a);
			/* If t1 != 1 and t1 != n - 1 return 0 */
			if (bn_cmp_dig(t1, 1) != CMP_EQ && bn_cmp(t1, t2) != CMP_EQ) {
				result = 0;
				break;
			}

			/* t2 = (t0|a). */
			bn_smb_jac(t2, t0, a);
			if (bn_sign(t2) == BN_NEG) {
				bn_add(t2, t2, a);
			}
			/* If t1 != t2 (mod a) return 0. */
			bn_mod_basic(t1, t1, a);
			bn_mod_basic(t2, t2, a);
			if (bn_cmp(t1, t2) != CMP_EQ) {
				result = 0;
				break;
			}
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
		return 0;
	}
	FINALLY {
		bn_free(t0);
		bn_free(t1);
		bn_free(t2);
	}
	return result;
}

#if BN_GEN == BASIC || !defined(STRIP)

void bn_gen_prime_basic(bn_t a, int bits) {
	while (1) {
		do {
			bn_rand(a, BN_POS, bits);
		} while (bn_bits(a) != bits);
		if (bn_is_prime(a)) {
			return;
		}
	}
}

#endif

#if BN_GEN == SAFEP || !defined(STRIP)

void bn_gen_prime_safep(bn_t a, int bits) {
	while (1) {
		do {
			bn_rand(a, BN_POS, bits);
		} while (bn_bits(a) != bits);
		if (bn_is_prime(a)) {
			/* Check if (a - 1)/2 is prime. */
			bn_sub_dig(a, a, 1);
			bn_rsh(a, a, 1);
			if (bn_is_prime(a)) {
				/* Restore a. */
				bn_lsh(a, a, 1);
				bn_add_dig(a, a, 1);
				return;
			}
		}
	}
}

#endif

#if BN_GEN == STRON || !defined(STRIP)

void bn_gen_prime_stron(bn_t a, int bits) {
	dig_t i, j;
	int found, k;
	bn_t r, s, t;

	bn_null(r);
	bn_null(s);
	bn_null(t);

	TRY {
		bn_new(r);
		bn_new(s);
		bn_new(t);

		do {
			do {
				/* Generate two large primes r and s. */
				bn_rand(s, BN_POS, bits / 2 - BN_DIGIT / 2);
				bn_rand(t, BN_POS, bits / 2 - BN_DIGIT / 2);
			} while (!bn_is_prime(s) || !bn_is_prime(t));
			found = 1;
			bn_rand(a, BN_POS, bits / 2 - bn_bits(t) - 1);
			i = a->dp[0];
			bn_dbl(t, t);
			do {
				/* Find first prime r = 2 * i * t + 1. */
				bn_mul_dig(r, t, i);
				bn_add_dig(r, r, 1);
				i++;
			} while (!bn_is_prime(r));
			if (bn_bits(r) != bits / 2 - 1) {
				found = 0;
				continue;
			}
			/* Compute t = 2 * (s^(r-2) mod r) * s - 1. */
			bn_sub_dig(t, r, 2);
			bn_exp(t, s, t, r);

			bn_mul(t, t, s);
			bn_dbl(t, t);
			bn_sub_dig(t, t, 1);

			k = bits - bn_bits(r);
			k -= bn_bits(s);
			bn_rand(a, BN_POS, k);
			j = a->dp[0];
			do {
				/* Find first prime a = t + 2 * j * r * s. */
				bn_mul(a, r, s);
				bn_mul_dig(a, a, j);
				bn_dbl(a, a);
				bn_add(a, a, t);
				j++;
			} while (!bn_is_prime(a));
			if (bn_bits(a) != bits) {
				found = 0;
				continue;
			}
		} while (found == 0);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(r);
		bn_free(s);
		bn_free(t);
		return;
	}
}

#endif
