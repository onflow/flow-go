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
 * Implementation of the multiple precision integer recoding functions.
 *
 * @version $Id$
 * @ingroup bn
 */

#include "relic_core.h"
#include "relic_bn.h"
#include "relic_bn_low.h"
#include "relic_error.h"
#include "relic_util.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

/**
 * Returns a maximum of eight contiguous bits from a multiple precision integer.
 *
 * @param[in] a				- the multiple precision integer.
 * @param[in] from			- the first bit position.
 * @param[in] to			- the last bit position, inclusive.
 * @return the bits in the chosen positions.
 */
static char get_bits(bn_t a, int from, int to) {
	int f, t;
	dig_t mf, mt;

	SPLIT(from, f, from, BN_DIG_LOG);
	SPLIT(to, t, to, BN_DIG_LOG);

	if (f == t) {
		/* Same digit. */

		mf = MASK(from);
		mt = MASK(to + 1);

		if (to + 1 == BN_DIGIT) {
			mt = DMASK;
		}

		mf = mf ^ mt;

		return ((a->dp[f] & (mf)) >> from);
	} else {
		mf = MASK(BN_DIGIT - from) << from;
		mt = MASK(to + 1);

		return ((a->dp[f] & mf) >> from) | ((a->dp[t] & mt) << (BN_DIGIT -
						from));
	}
}

/**
 * Constant C for the partial reduction modulo (t^m - 1)/(t - 1).
 */
#define MOD_C		8

/**
 * Constant 2^C.
 */
#define MOD_2TC		(1 << MOD_C)

/**
 * Mask to calculate reduction modulo 2^C.
 */
#define MOD_CMASK	(MOD_2TC - 1)

/**
 * Computes k partmod d = r0 + r1 * t, where d = (t^m - 1)/(t - 1).
 *
 * @param[out] r0		- the result.
 * @param[out] r1		- the result.
 * @param[in] k			- the number to reduce.
 * @param[in] vm		- the V_m curve parameter.
 * @param[in] s0		- the S0 curve parameter.
 * @param[in] s1		- the S1 curve parameter.
 * @param[in] u			- the u curve parameter.
 * @param[in] m			- the extension degree of the binary field.
 */
static void wtnaf_mod(bn_t r0, bn_t r1, bn_t k, bn_t vm, bn_t s0, bn_t s1,
		int u, int m) {
	bn_t t0, t1, t2, t3;
	int a, n, n0, n1, h0, h1;
	dig_t d0, d1;

	bn_null(t0);
	bn_null(t1);
	bn_null(t2);
	bn_null(t3);

	TRY {
		bn_new(t0);
		bn_new(t1);
		bn_new(t2);
		bn_new(t3);

		if (u == -1) {
			a = 0;
		} else {
			a = 1;
		}
		/* t2 = k' = floor(k/2^(a - C + (m - 9) / 2). */
		bn_rsh(t2, k, a - MOD_C + (m - 9) / 2);

		/* t0 = g' = s_i * k'. */
		bn_mul(t0, s0, t2);
		/* t3 = j' = V_m * floor(g'/2^m). */
		bn_rsh(t3, t0, m);
		bn_mul(t3, t3, vm);
		/* t3 = (g' + j'). */
		bn_add(t3, t3, t0);
		/* t0 = 1/2 * 2^(m + 5)/2. */
		bn_set_2b(t0, (m + 5) / 2 - 1);
		/* t3 = (g' + j' + 1/2 * 2^(m + 5)/2. */
		bn_add(t3, t3, t0);
		/* t0 = 2^C * l_i = floor((g' + j')/2^(m + 5)/2 + 1/2). */
		bn_rsh(t0, t3, (m + 5) / 2);

		/* t1 = g' = s_i * k'. */
		bn_mul(t1, s1, t2);

		/* t3 = j' = V_m * floor(g'/2^m). */
		bn_rsh(t3, t1, m);
		bn_mul(t3, t3, vm);
		/* t3 = (g' + j'). */
		bn_add(t3, t3, t1);
		/* t1 = 1/2 * 2^(m + 5)/2. */
		bn_set_2b(t1, (m + 5) / 2 - 1);
		/* t3 = (g' + j' + 1/2 * 2^(m + 5)/2. */
		bn_add(t3, t3, t1);
		/* t1 = 2^C * l_i = floor((g' + j')/2^(m + 5)/2 + 1/2). */
		bn_rsh(t1, t3, (m + 5) / 2);

		/*
		 * Both l0 and l1 should be rational, but we keep them as integers
		 * multiplied by 2^C. Now we can round off l0 and l1.
		 */

		/* Now we must round by f_i = floor(l_i + 1/2), n_i = l_i - f_i. */
		/* We must first pick the fractional part of l_i. */
		/* Since t_i = l_i * 2^C, the fractional part of l_i is t_i & (2^C - 1). */
		bn_get_dig(&d0, t0);
		n0 = d0 & MOD_CMASK;
		/* We must adjust  n_i because n_i is l_i - round(l_i). */
		/* if n_i > 0.5 * 2^C, n_i = (n_i - 1) * 2^C. */
		if (n0 >= MOD_2TC / 2) {
			n0 -= MOD_2TC;
		}
		/* If l_i is negative, we must negate n_i also. */
		if (bn_sign(t0) == BN_NEG) {
			n0 = -n0;
		}
		/* f_i = floor((l_i + 1/2) * 2^C). */
		bn_add_dig(t0, t0, MOD_2TC / 2);
		/* Restore q_i = f_i / 2^C. */
		bn_rsh(t0, t0, MOD_C);
		/* hi = 0. */
		h0 = 0;

		/* Now we repeat the same for q1. */
		bn_get_dig(&d1, t1);
		n1 = d1 & MOD_CMASK;
		if (n1 >= MOD_2TC / 2) {
			n1 -= MOD_2TC;
		}
		if (bn_cmp_dig(t1, 0) != CMP_GT) {
			n0 = -n0;
		}
		/* f_i = floor((l_i + 1/2) * 2^C). */
		bn_add_dig(t1, t1, MOD_2TC / 2);
		/* Restore q_i = f_i / 2^C. */
		bn_rsh(t1, t1, MOD_C);
		/* h_i = 0. */
		h1 = 0;

		/* n = 2n0 + u * n1. */
		n = 2 * n0 + u * n1;

		/* If n >= 1. */
		if (n >= MOD_2TC) {
			/* If n0 -3 * u * n1 < -1. */
			if (n0 - (3 * u * n1) < -MOD_2TC) {
				/* h1 = u. */
				h1 = u;
			} else {
				/* Else h0 = 1. */
				h0 = 1;
			}
		} else {
			/* Else if n0 + 4 * u * n1 >= 2. */
			if (n0 + 4 * u * n1 >= 2 * MOD_2TC) {
				h1 = u;
			}
		}

		/* If n < -1. */
		if (n < -MOD_2TC) {
			/* If n0 -3 * u * n1 >= 1. */
			if (n0 - 3 * u * n1 >= MOD_2TC) {
				/* h1 = -u. */
				h1 = -u;
			} else {
				/* Else h0 = -1. */
				h0 = -1;
			}
		} else {
			/* Else if n0 + 4 * u * n1 < -2. */
			if (n0 + 4 * u * n1 < -512) {
				/* h1 = -u. */
				h1 = -u;
			}
		}

		/* q0 = f0 + h0. */
		if (h0 == -1) {
			bn_sub_dig(t0, t0, 1);
		} else if (h0 == 1) {
			bn_add_dig(t0, t0, 1);
		}
		/* q1 = f1 + h1. */
		if (h1 == -1) {
			bn_sub_dig(t1, t1, 1);
		} else if (h1 == 1) {
			bn_add_dig(t1, t1, 1);
		}

		/* t2 = s0 + u * s1. */
		if (u == -1) {
			bn_sub(t2, s0, s1);
		} else {
			bn_add(t2, s0, s1);
		}

		/* t3 = (s0 + u * s1) * q0. */
		bn_mul(t3, t2, t0);
		/* t2 = k - (s0 + u * s1) * q0. */
		bn_sub(t2, k, t3);
		/* t3 = s1 * q1. */
		bn_mul(t3, s1, t1);
		/* t3 = 2 * s1 * q1. */
		bn_dbl(t3, t3);
		/* r0 = k - (s0 + u * s1) * q0 - 2 * s1 * q1. */
		bn_sub(r0, t2, t3);
		/* t2 = s1 * q0. */
		bn_mul(t2, s1, t0);
		/* t3 = s0 * q1. */
		bn_mul(t3, s0, t1);
		/* r1 = s1 * q0 - s0 * q1. */
		bn_sub(r1, t2, t3);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(t2);
		bn_free(t3);
		bn_free(t0);
		bn_free(t1);
	}
}

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void bn_rec_win(unsigned char *win, int *len, bn_t k, int w) {
	int i, l;

	l = bn_bits(k);

	l = ((l % w) == 0 ? (l / w) : (l / w) + 1);

	for (i = 0; i < l; i++) {
		win[i] = get_bits(k, i * w, (i + 1) * w - 1);
	}
	*len = l;
}

void bn_rec_slw(unsigned char *win, int *len, bn_t k, int w) {
	int i, j, s;

	i = bn_bits(k) - 1;
	j = 0;
	while (i >= 0) {
		if (!bn_test_bit(k, i)) {
			i--;
			win[j++] = 0;
		} else {
			s = MAX(i - w + 1, 0);
			while (!bn_test_bit(k, s)) {
				s++;
			}
			win[j++] = get_bits(k, s, i);
			i = s - 1;
		}
	}
	*len = j;
}

void bn_rec_naf(signed char *naf, int *len, bn_t k, int w) {
	int i, l;
	bn_t t;
	dig_t t0, mask;
	signed char u_i;

	bn_null(t);

	mask = MASK(w);
	l = (1 << w);

	TRY {
		bn_new(t);
		bn_copy(t, k);

		i = 0;
		if (w == 2) {
			while (!bn_is_zero(t)) {
				if (!bn_is_even(t)) {
					bn_get_dig(&t0, t);
					u_i = 2 - (t0 & MASK(2));
					if (u_i < 0) {
						bn_add_dig(t, t, -u_i);
					} else {
						bn_sub_dig(t, t, u_i);
					}
					*naf = u_i;
				} else {
					*naf = 0;
				}
				bn_hlv(t, t);
				i++;
				naf++;
			}
		} else {
			while (!bn_is_zero(t)) {
				if (!bn_is_even(t)) {
					bn_get_dig(&t0, t);
					u_i = t0 & mask;
					if (u_i > l / 2) {
						u_i = (signed char)(u_i - l);
					}
					if (u_i < 0) {
						bn_add_dig(t, t, -u_i);
					} else {
						bn_sub_dig(t, t, u_i);
					}
					*naf = u_i;
				} else {
					*naf = 0;
				}
				bn_hlv(t, t);
				i++;
				naf++;
			}
		}
		*len = i;
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(t);
	}
}

void bn_rec_tnaf(signed char *tnaf, int *len, bn_t k, bn_t vm, bn_t s0, bn_t s1,
		signed char u, int m, int w) {
	int i, l;
	bn_t tmp, r0, r1;
	signed char beta[16];
	signed char gama[16];
	dig_t t0, t1, t_w = 0, mask;
	signed char s, t, u_i;

	bn_null(r0);
	bn_null(r1);
	bn_null(tmp);

	TRY {
		bn_new(r0);
		bn_new(r1);
		bn_new(tmp);

		wtnaf_mod(r0, r1, k, vm, s0, s1, u, m);

		if (u == -1) {
			switch (w) {
				case 2:
				case 3:
					t_w = 2;
					break;
				case 4:
					t_w = 10;
					break;
				case 5:
				case 6:
					t_w = 26;
					break;
			}
		} else {
			switch (w) {
				case 2:
					t_w = 2;
					break;
				case 3:
				case 4:
				case 5:
					t_w = 6;
					break;
				case 6:
					t_w = 38;
					break;
			}
		}

		if (w >= 2) {
			beta[0] = 1;
			gama[0] = 0;
		}

		if (w >= 3) {
			beta[1] = 1;
			gama[1] = (signed char)-u;
		}

		if (w >= 4) {
			beta[1] = -3;
			beta[2] = -1;
			beta[3] = 1;
			gama[1] = gama[2] = gama[3] = (signed char)u;
		}

		if (w == 5) {
			beta[4] = -3;
			beta[5] = -1;
			beta[6] = beta[7] = 1;
			gama[4] = gama[5] = gama[6] = (signed char)(2 * u);
			gama[7] = (signed char)(-3 * u);
		}

		if (w == 6) {
			beta[1] = beta[8] = beta[14] = 3;
			beta[2] = beta[9] = beta[15] = 5;
			beta[3] = -5;
			beta[4] = beta[10] = beta[11] = -3;
			beta[5] = beta[12] = -1;
			beta[6] = beta[7] = beta[13] = 1;
			gama[1] = gama[2] = 0;
			gama[3] = gama[4] = gama[5] = gama[6] = (signed char)(2 * u);
			gama[7] = gama[8] = gama[9] = (signed char)(-3 * u);
			gama[10] = (signed char)(4 * u);
			gama[11] = gama[12] = gama[13] = gama[14] = gama[15] =
					(signed char)(-u);
		}

		mask = MASK(w);
		l = 1 << w;

		i = 0;
		while (!bn_is_zero(r0) || !bn_is_zero(r1)) {
			/* If r0 is odd. */
			if (!bn_is_even(r0)) {
				if (w == 2) {
					bn_get_dig(&t0, r0);
					if (bn_sign(r0) == BN_NEG) {
						t0 = l - t0;
					}
					bn_get_dig(&t1, r1);
					if (bn_sign(r1) == BN_NEG) {
						t1 = l - t1;
					}
					u_i = 2 - ((t0 - 2 * t1) & mask);
					*tnaf = u_i;
					if (u_i < 0) {
						bn_add_dig(r0, r0, -u_i);
					} else {
						bn_sub_dig(r0, r0, u_i);
					}
				} else {
					/* t0 = r0 mod_s 2^w. */
					bn_get_dig(&t0, r0);
					t0 = t0 & mask;
					if (bn_sign(r0) == BN_NEG) {
						t0 = l - t0;
					}
					/* t1 = r1 mod_s 2^w. */
					bn_get_dig(&t1, r1);
					t1 = t1 & mask;
					if (bn_sign(r1) == BN_NEG) {
						t1 = l - t1;
					}
					/* u = r0 + r1 * (t_w) mod_s 2^w. */
					u_i = (t0 + t_w * t1) & mask;
					if (u_i >= (l / 2)) {
						u_i = (signed char)(u_i - l);
					}
					*tnaf = u_i;
					/* If u > 0, s = 1. */
					if (u_i > 0) {
						s = 1;
						u_i = (signed char)(u_i >> 1);
					} else {
						/* Else s = -1 and u = -u. */
						s = -1;
						u_i = (signed char)(-u_i >> 1);
					}
					/* r0 = r0 - s * beta_u. */
					t = (signed char)(s * beta[(int)u_i]);
					if (t > 0) {
						bn_sub_dig(r0, r0, t);
					} else {
						bn_add_dig(r0, r0, -t);
					}
					/* r1 = r1 - s * gama_u. */
					t = (signed char)(s * gama[(int)u_i]);
					if (t > 0) {
						bn_sub_dig(r1, r1, t);
					} else {
						bn_add_dig(r1, r1, -t);
					}
				}
			} else {
				/* Else u_i = 0. */
				*tnaf = 0;
			}
			/* tmp = r0. */
			bn_hlv(tmp, r0);
			/* r0 = r1 + mu * r0 / 2. */
			if (u == -1) {
				bn_sub(r0, r1, tmp);
			} else {
				bn_add(r0, r1, tmp);
			}
			/* r1 = - r0 / 2. */
			bn_neg(r1, tmp);
			tnaf++;
			i++;
		}
		*len = i;
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(r0);
		bn_free(r1);
		bn_free(tmp);
	}
}

void bn_rec_jsf(signed char *jsf, int *len, bn_t k, bn_t l) {
	bn_t n0, n1;
	dig_t l0, l1;
	signed char u0, u1, d0, d1;
	int i, j, offset;

	bn_null(n0);
	bn_null(n1);

	TRY {
		bn_new(n0);
		bn_new(n1);

		bn_copy(n0, k);
		bn_copy(n1, l);

		i = bn_bits(k);
		j = bn_bits(l);
		offset = MAX(i, j) + 1;

		i = 0;
		d0 = d1 = 0;
		while (!(bn_is_zero(n0) && d0 == 0) || !(bn_is_zero(n1) && d1 == 0)) {
			bn_get_dig(&l0, n0);
			bn_get_dig(&l1, n1);
			/* For reduction modulo 8. */
			l0 = (l0 + d0) & MASK(3);
			l1 = (l1 + d1) & MASK(3);

			if (l0 % 2 == 0) {
				u0 = 0;
			} else {
				u0 = 2 - (l0 & MASK(2));
				if ((l0 == 3 || l0 == 5) && ((l1 & MASK(2)) == 2)) {
					u0 = (signed char)-u0;
				}
			}
			jsf[i] = u0;
			if (l1 % 2 == 0) {
				u1 = 0;
			} else {
				u1 = 2 - (l1 & MASK(2));
				if ((l1 == 3 || l1 == 5) && ((l0 & MASK(2)) == 2)) {
					u1 = (signed char)-u1;
				}
			}
			jsf[i + offset] = u1;

			if (d0 + d0 == 1 + u0) {
				d0 = (signed char)(1 - d0);
			}
			if (d1 + d1 == 1 + u1) {
				d1 = (signed char)(1 - d1);
			}

			i++;
			bn_hlv(n0, n0);
			bn_hlv(n1, n1);
		}
		*len = i;
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(n0);
		bn_free(n1);
	}

}
