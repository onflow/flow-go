/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2012 RELIC Authors
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
 * Implementation of the binary elliptic curve utilities.
 *
 * @version $Id$
 * @ingroup eb
 */

#include "relic_core.h"
#include "relic_eb.h"
#include "relic_error.h"
#include "relic_conf.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

int eb_is_infty(eb_t p) {
	return (fb_is_zero(p->z) == 1);
}

void eb_set_infty(eb_t p) {
	fb_zero(p->x);
	fb_zero(p->y);
	fb_zero(p->z);
	p->norm = 1;
}

void eb_copy(eb_t r, eb_t p) {
	fb_copy(r->x, p->x);
	fb_copy(r->y, p->y);
	fb_copy(r->z, p->z);
	r->norm = p->norm;
}

int eb_cmp(eb_t p, eb_t q) {
	if (fb_cmp(p->x, q->x) != CMP_EQ) {
		return CMP_NE;
	}

	if (fb_cmp(p->y, q->y) != CMP_EQ) {
		return CMP_NE;
	}

	if (fb_cmp(p->z, q->z) != CMP_EQ) {
		return CMP_NE;
	}

	return CMP_EQ;
}

void eb_rand(eb_t p) {
	bn_t n, k;

	bn_null(n);
	bn_null(k);

	TRY {
		bn_new(k);
		bn_new(n);

		eb_curve_get_ord(n);

		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);

		eb_mul_gen(p, k);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		bn_free(k);
		bn_free(n);
	}
}

void eb_rhs(fb_t rhs, eb_t p) {
	fb_t t0, t1;

	fb_null(t0);
	fb_null(t1);

	TRY {
		fb_new(t0);
		fb_new(t1);

		/* t0 = x1^2. */
		fb_sqr(t0, p->x);
		/* t1 = x1^3. */
		fb_mul(t1, t0, p->x);

		if (eb_curve_is_super()) {
			/* t1 = x1^3 + a * x1 + b. */
			switch (eb_curve_opt_a()) {
				case OPT_ZERO:
					break;
				case OPT_ONE:
					fb_add(t1, t1, p->x);
					break;
				case OPT_DIGIT:
					fb_mul_dig(t0, p->x, eb_curve_get_a()[0]);
					fb_add(t1, t1, t0);
					break;
				default:
					fb_mul(t0, p->x, eb_curve_get_a());
					fb_add(t1, t1, t0);
					break;
			}
		} else {
			/* t1 = x1^3 + a * x1^2 + b. */
			switch (eb_curve_opt_a()) {
				case OPT_ZERO:
					break;
				case OPT_ONE:
					fb_add(t1, t1, t0);
					break;
				case OPT_DIGIT:
					fb_mul_dig(t0, t0, eb_curve_get_a()[0]);
					fb_add(t1, t1, t0);
					break;
				default:
					fb_mul(t0, t0, eb_curve_get_a());
					fb_add(t1, t1, t0);
					break;
			}
		}

		switch (eb_curve_opt_b()) {
			case OPT_ZERO:
				break;
			case OPT_ONE:
				fb_add_dig(t1, t1, 1);
				break;
			case OPT_DIGIT:
				fb_add_dig(t1, t1, eb_curve_get_b()[0]);
				break;
			default:
				fb_add(t1, t1, eb_curve_get_b());
				break;
		}

		fb_copy(rhs, t1);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb_free(t0);
		fb_free(t1);
	}
}

int eb_is_valid(eb_t p) {
	eb_t t;
	fb_t lhs;
	int r = 0;

	eb_null(t);
	fb_null(lhs);

	TRY {
		eb_new(t);
		fb_new(lhs);

		eb_norm(t, p);

		if (eb_curve_is_super()) {
			fb_mul(lhs, t->y, eb_curve_get_c());
		} else {
			fb_mul(lhs, t->x, t->y);
		}
		eb_rhs(t->x, t);
		fb_sqr(t->y, t->y);
		fb_add(lhs, lhs, t->y);
		r = (fb_cmp(lhs, t->x) == CMP_EQ) || eb_is_infty(p);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		eb_free(t);
		fb_free(lhs);
	}
	return r;
}

void eb_mul_frb(eb_t r, eb_t p, signed char *k, int l) {
	int i;
	eb_t t;

	eb_null(t);

	TRY {
		eb_new(t);

		if (k[l - 1] >= 0) {
			eb_copy(t, p);
		} else {
			eb_neg(t, p);
		}
		for (i = l - 2; i >= 0; i--) {
			eb_frb(t, t);
			if (k[i] > 0) {
				eb_add(t, t, p);
			}
			if (k[i] < 0) {
				eb_sub(t, t, p);
			}
		}

		eb_norm(r, t);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		eb_free(t);
	}
}

void eb_tab(eb_t *t, eb_t p, int w) {
	int u;

#if defined(EB_ORDIN) || defined(EB_SUPER)
	if (!eb_curve_is_kbltz()) {
		if (w > 2) {
			eb_dbl(t[0], p);
#if defined(EB_MIXED)
			eb_norm(t[0], t[0]);
#endif
			eb_add(t[1], t[0], p);
			for (int i = 2; i < (1 << (w - 2)); i++) {
				eb_add(t[i], t[i - 1], t[0]);
			}
#if defined(EB_MIXED)
			eb_norm_sim(t + 1, t + 1, (1 << (w - 2)) - 1);
#endif
		}
		eb_copy(t[0], p);
	}
#endif /* EB_ORDIN || EB_SUPER */

#if defined(EB_KBLTZ)
	if (eb_curve_is_kbltz()) {
		u = (eb_curve_opt_a() == OPT_ZERO ? -1 : 1);

		/* Prepare the precomputation table. */
		for (int i = 0; i < 1 << (w - 2); i++) {
			eb_set_infty(t[i]);
			fb_set_dig(t[i]->z, 1);
			t[i]->norm = 1;
		}

		eb_copy(t[0], p);
#if defined(EB_MIXED)
		eb_norm(t[0], t[0]);
#endif

		switch (w) {
#if EB_DEPTH == 3 || EB_WIDTH ==  3
			case 3:
				eb_frb(t[1], t[0]);
				if (u == 1) {
					eb_sub(t[1], t[0], t[1]);
				} else {
					eb_add(t[1], t[0], t[1]);
				}
				break;
#endif
#if EB_DEPTH == 4 || EB_WIDTH ==  4
			case 4:
				eb_frb(t[3], t[0]);
				eb_frb(t[3], t[3]);

				eb_sub(t[1], t[3], p);
				eb_add(t[2], t[3], p);
				eb_frb(t[3], t[3]);

				if (u == 1) {
					eb_neg(t[3], t[3]);
				}
				eb_sub(t[3], t[3], p);
				break;
#endif
#if EB_DEPTH == 5 || EB_WIDTH ==  5
			case 5:
				eb_frb(t[3], t[0]);
				eb_frb(t[3], t[3]);

				eb_sub(t[1], t[3], p);
				eb_add(t[2], t[3], p);
				eb_frb(t[3], t[3]);

				if (u == 1) {
					eb_neg(t[3], t[3]);
				}
				eb_sub(t[3], t[3], p);

				eb_frb(t[4], t[2]);
				eb_frb(t[4], t[4]);

#if defined(EB_MIXED) && defined(STRIP)
				eb_norm(t[2], t[2]);
#endif
				eb_sub(t[7], t[4], t[2]);

				eb_neg(t[4], t[4]);
				eb_sub(t[5], t[4], p);
				eb_add(t[6], t[4], p);

				eb_frb(t[4], t[4]);
				if (u == -1) {
					eb_neg(t[4], t[4]);
				}
				eb_add(t[4], t[4], p);
				break;
#endif
#if EB_DEPTH == 6 || EB_WIDTH ==  6
			case 6:
				eb_frb(t[0], t[0]);
				eb_frb(t[0], t[0]);
				eb_neg(t[14], t[0]);

				eb_sub(t[13], t[14], p);
				eb_add(t[14], t[14], p);

				eb_frb(t[0], t[0]);
				if (u == -1) {
					eb_neg(t[0], t[0]);
				}
				eb_sub(t[11], t[0], p);
				eb_add(t[12], t[0], p);

				eb_frb(t[0], t[12]);
				eb_frb(t[0], t[0]);
				eb_sub(t[1], t[0], p);
				eb_add(t[2], t[0], p);

#if defined(EB_MIXED) && defined(STRIP)
				eb_norm(t[13], t[13]);
#endif
				eb_add(t[15], t[0], t[13]);

				eb_frb(t[0], t[13]);
				eb_frb(t[0], t[0]);
				eb_sub(t[5], t[0], p);
				eb_add(t[6], t[0], p);

				eb_neg(t[8], t[0]);
				eb_add(t[7], t[8], t[13]);
#if defined(EB_MIXED) && defined(STRIP)
				eb_norm(t[14], t[14]);
#endif
				eb_add(t[8], t[8], t[14]);

				eb_frb(t[0], t[0]);
				if (u == -1) {
					eb_neg(t[0], t[0]);
				}
				eb_sub(t[3], t[0], p);
				eb_add(t[4], t[0], p);

				eb_frb(t[0], t[1]);
				eb_frb(t[0], t[0]);

				eb_neg(t[9], t[0]);
				eb_sub(t[9], t[9], p);

				eb_frb(t[0], t[14]);
				eb_frb(t[0], t[0]);
				eb_add(t[10], t[0], p);

				eb_copy(t[0], p);
				break;
#endif
#if EB_DEPTH == 7 || EB_WIDTH ==  7
			case 7: {
				signed char tnaf[64] = { 0 }, beta[64], gama[64];
				int u, u_i, len;
				dig_t t0, t1, mask;
				bn_t k, vm, s0, s1, r0, r1, tmp;
				bn_new(k);
				bn_new(vm);
				bn_new(s0);
				bn_new(s1);
				bn_new(tmp);
				bn_new(r0);
				bn_new(r1);

				if (eb_curve_opt_a() == OPT_ZERO) {
					u = -1;
				} else {
					u = 1;
				}

				eb_curve_get_vm(vm);
				eb_curve_get_s0(s0);
				eb_curve_get_s1(s1);

				beta[0] = 1;
				gama[0] = 0;

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

				if (w >= 5) {
					beta[4] = -3;
					beta[5] = -1;
					beta[6] = beta[7] = 1;
					gama[4] = gama[5] = gama[6] = (signed char)(2 * u);
					gama[7] = (signed char)(-3 * u);
				}

				if (w >= 6) {
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
					gama[11] = gama[12] = gama[13] = (signed char)(-u);
					gama[14] = gama[15] = (signed char)(-u);
				}

				if (w >= 7) {
					beta[3] = beta[22] = beta[29] = 7;
					beta[4] = beta[16] = beta[23] = -5;
					beta[5] = beta[10] = beta[17] = beta[24] = -3;
					beta[6] = beta[11] = beta[18] = beta[25] = beta[30] = -1;
					beta[7] = beta[12] = beta[14] = beta[19] = beta[26] = beta[31] = 1;
					beta[8] = beta[13] = beta[20] = beta[27] = 3;
					beta[9] = beta[21] = beta[28] = 5;
					beta[15] = -7;
					gama[3] = 0;
					gama[4] = gama[5] = gama[6] = (signed char)(-3 * u);
					gama[11] = gama[12] = gama[13] = (signed char)(4 * u);
					gama[14] = (signed char)(-6 * u);
					gama[15] = gama[16] = gama[17] = gama[18] = (signed char)u;
					gama[19] = gama[20] = gama[21] = gama[22] = (signed char)u;
					gama[23] = gama[24] = gama[25] = gama[26] = (signed char)(-2 * u);
					gama[27] = gama[28] = gama[29] = (signed char)(-2 * u);
					gama[30] = gama[31] = (signed char)(5 * u);
				}

				if (w == 8) {
					beta[10] = beta[17] = beta[48] = beta[55] = beta[62] = 7;
					beta[11] = beta[18] = beta[49] = beta[56] = beta[63] = 9;
					beta[12] = beta[22] = beta[29] = -3;
					beta[36] = beta[43] = beta[50] = -3;
					beta[13] = beta[23] = beta[30] = beta[37] = -1;
					beta[44] = beta[51] = beta[58] = -1;
					beta[14] = beta[24] = beta[31] = beta[38] = 1;
					beta[45] = beta[52] = beta[59] = 1;
					beta[15] = beta[32] = beta[39] = beta[46] = beta[53] = beta[60] = 3;
					beta[16] = beta[40] = beta[47] = beta[54] = beta[61] = 5;
					beta[19] = beta[57] = 11;
					beta[20] = beta[27] = beta[34] = beta[41] = -7;
					beta[21] = beta[28] = beta[35] = beta[42] = -5;
					beta[25] = -11;
					beta[26] = beta[33] = -9;
					gama[10] = gama[11] = (signed char)(-3 * u);
					gama[12] = gama[13] = gama[14] = gama[15] = (signed char)(-6 * u);
					gama[16] = gama[17] = gama[18] = gama[19] = (signed char)(-6 * u);
					gama[20] = gama[21] = gama[22] = (signed char)(8 * u);
					gama[23] = gama[24] = (signed char)(8 * u);
					gama[25] = gama[26] = gama[27] = gama[28] = (signed char)(5 * u);
					gama[29] = gama[30] = gama[31] = gama[32] = (signed char)(5 * u);
					gama[33] = gama[34] = gama[35] = gama[36] = (signed char)(2 * u);
					gama[37] = gama[38] = gama[39] = gama[40] = (signed char)(2 * u);
					gama[41] = gama[42] = gama[43] = gama[44] = (signed char)(-1 * u);
					gama[45] = gama[46] = gama[47] = gama[48] = (signed char)(-1 * u);
					gama[49] = (signed char)(-1 * u);
					gama[50] = gama[51] = gama[52] = gama[53] = (signed char)(-4 * u);
					gama[54] = gama[55] = gama[56] = gama[57] = (signed char)(-4 * u);
					gama[58] = gama[59] = gama[60] = (signed char)(-7 * u);
					gama[61] = gama[62] = gama[63] = (signed char)(-7 * u);
				}

				for (int _k = 0; _k < 32; _k++) {
					if (beta[_k] < 0) {
						bn_set_dig(r0, -beta[_k]);
						bn_neg(r0, r0);
					} else {
						bn_set_dig(r0, beta[_k]);
					}
					if (gama[_k] < 0) {
						bn_set_dig(r1, -gama[_k]);
						bn_neg(r1, r1);
					} else {
						bn_set_dig(r1, gama[_k]);
					}

					/*printf("$");
					if (gama[_k] == -1) {
						printf("-\\tau");
					}
					if (gama[_k] == 1) {
						printf("\\tau");
					}
					if (gama[_k] < -1 || gama[_k] > 1) {
						printf("%d\\tau", gama[_k]);
					}
					if (beta[_k] > 0) {
						printf("+ %d$\n", beta[_k]);
					} else {
						printf("- %d$\n", -beta[_k]);
					}*/

					mask = MASK(2);
					int l = 1 << 2;

					int i = 0;
					while (!bn_is_zero(r0) || !bn_is_zero(r1)) {
						while ((r0->dp[0] & 1) == 0) {
							tnaf[i++] = 0;
							/* tmp = r0. */
							bn_hlv(tmp, r0);
							/* r0 = r1 + mu * r0 / 2. */
							if (u == -1) {
								bn_sub(r0, r1, tmp);
							} else {
								bn_add(r0, r1, tmp);
							}
							/* r1 = - r0 / 2. */
							bn_copy(r1, tmp);
							r1->sign = tmp->sign ^ 1;
						}
						/* If r0 is odd. */
						t0 = r0->dp[0];
						if (bn_sign(r0) == BN_NEG) {
							t0 = l - t0;
						}
						t1 = r1->dp[0];
						if (bn_sign(r1) == BN_NEG) {
							t1 = l - t1;
						}
						u_i = 2 - ((t0 - 2 * t1) & mask);
						tnaf[i++] = u_i;
						if (u_i < 0) {
							bn_add_dig(r0, r0, -u_i);
						} else {
							bn_sub_dig(r0, r0, u_i);
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
						bn_copy(r1, tmp);
						r1->sign = tmp->sign ^ 1;
					}
					len = i;

					printf("(");
					for (int j = len - 1; j >= 0; j--) {
						if (j > 0) {
							printf("%d, ", tnaf[j]);
						} else {
							printf("%d)", tnaf[j]);
						}
					}
					printf("\n");
					eb_mul_frb(t[_k], p, tnaf, len);
				}
/*
				signed char k1[] = {-1, 0, 1, 0, 0, -1, };
				eb_mul_frb(t[1], p, k1, sizeof(k1));
				signed char k2[] = {1, 0, 1, 0, 0, -1, };
				eb_mul_frb(t[2], p, k2, sizeof(k2));
				signed char k3[] = {-1, 0, 0, 1, 0, -1, };
				eb_mul_frb(t[3], p, k3, sizeof(k3));
				signed char k4[] = {1, 0, 0, 1, 0, -1, 0, 0, 1, };
				eb_mul_frb(t[4], p, k4, sizeof(k4));
				signed char k5[] = {-1, 0, -1, 0, -1, 0, -1, };
				eb_mul_frb(t[5], p, k5, sizeof(k5));
				signed char k6[] = {1, 0, -1, 0, -1, 0, -1, };
				eb_mul_frb(t[6], p, k6, sizeof(k6));
				signed char k7[] = {-1, 0, 0, 0, 1, };
				eb_mul_frb(t[7], p, k7, sizeof(k7));
				signed char k8[] = {1, 0, 0, 0, 1, };
				eb_mul_frb(t[8], p, k8, sizeof(k8));
				signed char k9[] = {-1, 0, 1, 0, -1, 0, 0, 1, };
				eb_mul_frb(t[9], p, k9, sizeof(k9));
				signed char k10[] = {1, 0, 1, 0, -1, };
				eb_mul_frb(t[10], p, k10, sizeof(k10));
				signed char k11[] = {-1, 0, 0, -1, 0, 0, 1, };
				eb_mul_frb(t[11], p, k11, sizeof(k11));
				signed char k12[] = {1, 0, 0, -1, 0, 0, 1, };
				eb_mul_frb(t[12], p, k12, sizeof(k12));
				signed char k13[] = {-1, 0, -1, 0, 0, 0, 1, };
				eb_mul_frb(t[13], p, k13, sizeof(k13));
				signed char k14[] = {1, 0, -1, 0, 0, 0, -1, };
				eb_mul_frb(t[14], p, k14, sizeof(k14));
				signed char k15[] = {-1, 0, 0, 0, 0, 1, };
				eb_mul_frb(t[15], p, k15, sizeof(k15));
				signed char k16[] = {1, 0, 0, 0, 0, 1, };
				eb_mul_frb(t[16], p, k16, sizeof(k16));
				signed char k17[] = {-1, 0, 1, };
				eb_mul_frb(t[17], p, k17, sizeof(k17));
				signed char k18[] = {1, 0, 1, };
				eb_mul_frb(t[18], p, k18, sizeof(k18));
				signed char k19[] = {-1, 0, 0, 1, };
				eb_mul_frb(t[19], p, k19, sizeof(k19));
				signed char k20[] = {1, 0, 0, 1, };
				eb_mul_frb(t[20], p, k20, sizeof(k20));
				signed char k21[] = {-1, 0, -1, 0, 1, 0, 1, };
				eb_mul_frb(t[21], p, k21, sizeof(k21));
				signed char k22[] = {1, 0, -1, 0, 1, 0, 1, };
				eb_mul_frb(t[22], p, k22, sizeof(k22));
				signed char k23[] = {-1, 0, 0, 0, -1, 0, -1, };
				eb_mul_frb(t[23], p, k23, sizeof(k23));
				signed char k24[] = {1, 0, 0, 0, -1, 0, -1, };
				eb_mul_frb(t[24], p, k24, sizeof(k24));
				signed char k25[] = {-1, 0, 1, 0, 1, };
				eb_mul_frb(t[25], p, k25, sizeof(k25));
				signed char k26[] = {1, 0, 1, 0, 1, };
				eb_mul_frb(t[26], p, k26, sizeof(k26));
				signed char k27[] =  {-1, 0, 0, -1, 0, -1, };
				eb_mul_frb(t[27], p, k27, sizeof(k27));
				signed char k28[] = {1, 0, 0, -1, 0, -1, };
				eb_mul_frb(t[28], p, k28, sizeof(k28));
				signed char k29[] = {-1, 0, -1, 0, 0, -1, };
				eb_mul_frb(t[29], p, k29, sizeof(k29));
				signed char k30[] = {1, 0, -1, 0, 0, -1, 0, -1, };
				eb_mul_frb(t[30], p, k30, sizeof(k30));
				signed char k31[] = {-1, 0, 0, 0, 0, 0, 1, };
				eb_mul_frb(t[31], p, k31, sizeof(k31));*/
				eb_copy(t[0], p);
			}
			break;
#endif
#if EB_DEPTH == 8 || EB_WIDTH ==  8
			case 8: {
				signed char tnaf[64] = { 0 }, beta[64], gama[64];
				int u, u_i, len;
				dig_t t0, t1, mask;
				bn_t k, vm, s0, s1, r0, r1, tmp;
				bn_new(k);
				bn_new(vm);
				bn_new(s0);
				bn_new(s1);
				bn_new(tmp);
				bn_new(r0);
				bn_new(r1);

				if (eb_curve_opt_a() == OPT_ZERO) {
					u = -1;
				} else {
					u = 1;
				}

				eb_curve_get_vm(vm);
				eb_curve_get_s0(s0);
				eb_curve_get_s1(s1);

				beta[0] = 1;
				gama[0] = 0;

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

				if (w >= 5) {
					beta[4] = -3;
					beta[5] = -1;
					beta[6] = beta[7] = 1;
					gama[4] = gama[5] = gama[6] = (signed char)(2 * u);
					gama[7] = (signed char)(-3 * u);
				}

				if (w >= 6) {
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
					gama[11] = gama[12] = gama[13] = (signed char)(-u);
					gama[14] = gama[15] = (signed char)(-u);
				}

				if (w >= 7) {
					beta[3] = beta[22] = beta[29] = 7;
					beta[4] = beta[16] = beta[23] = -5;
					beta[5] = beta[10] = beta[17] = beta[24] = -3;
					beta[6] = beta[11] = beta[18] = beta[25] = beta[30] = -1;
					beta[7] = beta[12] = beta[14] = beta[19] = beta[26] = beta[31] = 1;
					beta[8] = beta[13] = beta[20] = beta[27] = 3;
					beta[9] = beta[21] = beta[28] = 5;
					beta[15] = -7;
					gama[3] = 0;
					gama[4] = gama[5] = gama[6] = (signed char)(-3 * u);
					gama[11] = gama[12] = gama[13] = (signed char)(4 * u);
					gama[14] = (signed char)(-6 * u);
					gama[15] = gama[16] = gama[17] = gama[18] = (signed char)u;
					gama[19] = gama[20] = gama[21] = gama[22] = (signed char)u;
					gama[23] = gama[24] = gama[25] = gama[26] = (signed char)(-2 * u);
					gama[27] = gama[28] = gama[29] = (signed char)(-2 * u);
					gama[30] = gama[31] = (signed char)(5 * u);
				}

				if (w == 8) {
					beta[10] = beta[17] = beta[48] = beta[55] = beta[62] = 7;
					beta[11] = beta[18] = beta[49] = beta[56] = beta[63] = 9;
					beta[12] = beta[22] = beta[29] = -3;
					beta[36] = beta[43] = beta[50] = -3;
					beta[13] = beta[23] = beta[30] = beta[37] = -1;
					beta[44] = beta[51] = beta[58] = -1;
					beta[14] = beta[24] = beta[31] = beta[38] = 1;
					beta[45] = beta[52] = beta[59] = 1;
					beta[15] = beta[32] = beta[39] = beta[46] = beta[53] = beta[60] = 3;
					beta[16] = beta[40] = beta[47] = beta[54] = beta[61] = 5;
					beta[19] = beta[57] = 11;
					beta[20] = beta[27] = beta[34] = beta[41] = -7;
					beta[21] = beta[28] = beta[35] = beta[42] = -5;
					beta[25] = -11;
					beta[26] = beta[33] = -9;
					gama[10] = gama[11] = (signed char)(-3 * u);
					gama[12] = gama[13] = gama[14] = gama[15] = (signed char)(-6 * u);
					gama[16] = gama[17] = gama[18] = gama[19] = (signed char)(-6 * u);
					gama[20] = gama[21] = gama[22] = (signed char)(8 * u);
					gama[23] = gama[24] = (signed char)(8 * u);
					gama[25] = gama[26] = gama[27] = gama[28] = (signed char)(5 * u);
					gama[29] = gama[30] = gama[31] = gama[32] = (signed char)(5 * u);
					gama[33] = gama[34] = gama[35] = gama[36] = (signed char)(2 * u);
					gama[37] = gama[38] = gama[39] = gama[40] = (signed char)(2 * u);
					gama[41] = gama[42] = gama[43] = gama[44] = (signed char)(-1 * u);
					gama[45] = gama[46] = gama[47] = gama[48] = (signed char)(-1 * u);
					gama[49] = (signed char)(-1 * u);
					gama[50] = gama[51] = gama[52] = gama[53] = (signed char)(-4 * u);
					gama[54] = gama[55] = gama[56] = gama[57] = (signed char)(-4 * u);
					gama[58] = gama[59] = gama[60] = (signed char)(-7 * u);
					gama[61] = gama[62] = gama[63] = (signed char)(-7 * u);
				}

				for (int _k = 0; _k < 64; _k++) {
					if (beta[_k] < 0) {
						bn_set_dig(r0, -beta[_k]);
						bn_neg(r0, r0);
					} else {
						bn_set_dig(r0, beta[_k]);
					}
					if (gama[_k] < 0) {
						bn_set_dig(r1, -gama[_k]);
						bn_neg(r1, r1);
					} else {
						bn_set_dig(r1, gama[_k]);
					}

					/*printf("$");
					if (gama[_k] == -1) {
						printf("-\\tau");
					}
					if (gama[_k] == 1) {
						printf("\\tau");
					}
					if (gama[_k] < -1 || gama[_k] > 1) {
						printf("%d\\tau", gama[_k]);
					}
					if (beta[_k] > 0) {
						printf("+ %d$\n", beta[_k]);
					} else {
						printf("- %d$\n", -beta[_k]);
					}*/

					mask = MASK(2);
					int l = 1 << 2;

					int i = 0;
					while (!bn_is_zero(r0) || !bn_is_zero(r1)) {
						while ((r0->dp[0] & 1) == 0) {
							tnaf[i++] = 0;
							/* tmp = r0. */
							bn_hlv(tmp, r0);
							/* r0 = r1 + mu * r0 / 2. */
							if (u == -1) {
								bn_sub(r0, r1, tmp);
							} else {
								bn_add(r0, r1, tmp);
							}
							/* r1 = - r0 / 2. */
							bn_copy(r1, tmp);
							r1->sign = tmp->sign ^ 1;
						}
						/* If r0 is odd. */
						t0 = r0->dp[0];
						if (bn_sign(r0) == BN_NEG) {
							t0 = l - t0;
						}
						t1 = r1->dp[0];
						if (bn_sign(r1) == BN_NEG) {
							t1 = l - t1;
						}
						u_i = 2 - ((t0 - 2 * t1) & mask);
						tnaf[i++] = u_i;
						if (u_i < 0) {
							bn_add_dig(r0, r0, -u_i);
						} else {
							bn_sub_dig(r0, r0, u_i);
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
						bn_copy(r1, tmp);
						r1->sign = tmp->sign ^ 1;
					}
					len = i;

					/*printf("(");
					for (int j = len - 1; j >= 0; j--) {
						if (j > 0) {
							printf("%d, ", tnaf[j]);
						} else {
							printf("%d)", tnaf[j]);
						}
					}
					printf("\n");*/
					eb_mul_frb(t[_k], p, tnaf, len);
				}

/*				signed char k1[] = {-1, 0, 1, 0, 0, -1, };;
				eb_mul_frb(t[1], p, k1, sizeof(k1));
				signed char k2[] = {1, 0, 1, 0, 0, -1, };
				eb_mul_frb(t[2], p, k2, sizeof(k2));
				signed char k3[] = {-1, 0, 0, 1, 0, -1, };
				eb_mul_frb(t[3], p, k3, sizeof(k3));
				signed char k4[] = {1, 0, 0, 1, 0, -1, 0, 0, 1, };
				eb_mul_frb(t[4], p, k4, sizeof(k4));
				signed char k5[] = {-1, 0, -1, 0, -1, 0, -1, };
				eb_mul_frb(t[5], p, k5, sizeof(k5));
				signed char k6[] = {1, 0, -1, 0, -1, 0, -1, };
				eb_mul_frb(t[6], p, k6, sizeof(k6));
				signed char k7[] = {-1, 0, 0, 0, 1, };
				eb_mul_frb(t[7], p, k7, sizeof(k7));
				signed char k8[] = {1, 0, 0, 0, 1, };
				eb_mul_frb(t[8], p, k8, sizeof(k8));
				signed char k9[] = {-1, 0, 1, 0, -1, 0, 0, 1, };
				eb_mul_frb(t[9], p, k9, sizeof(k9));
				signed char k10[] = {1, 0, 1, 0, -1, 0, 0, 1, };
				eb_mul_frb(t[10], p, k10, sizeof(k10));
				signed char k11[] = {-1, 0, 0, -1, 0, 0, -1, 0, -1, };
				eb_mul_frb(t[11], p, k11, sizeof(k11));
				signed char k12[] = {1, 0, 0, -1, 0, 0, -1, };
				eb_mul_frb(t[12], p, k12, sizeof(k12));
				signed char k13[] = {-1, 0, -1, 0, 0, 0, -1, };
				eb_mul_frb(t[13], p, k13, sizeof(k13));
				signed char k14[] = {1, 0, -1, 0, 0, 0, -1, };
				eb_mul_frb(t[14], p, k14, sizeof(k14));
				signed char k15[] = {-1, 0, 0, 0, 0, 1, 0, 1, };
				eb_mul_frb(t[15], p, k15, sizeof(k15));
				signed char k16[] = {1, 0, 0, 0, 0, 1, 0, 1, };
				eb_mul_frb(t[16], p, k16, sizeof(k16));
				signed char k17[] = {-1, 0, 1, 0, 0, 0, 0, 1, };
				eb_mul_frb(t[17], p, k17, sizeof(k17));
				signed char k18[] = {1, 0, 1, 0, 0, 0, 0, 1, };
				eb_mul_frb(t[18], p, k18, sizeof(k18));
				signed char k19[] = {-1, 0, 0, 1, 0, 0, 0, 1, };
				eb_mul_frb(t[19], p, k19, sizeof(k19));
				signed char k20[] = {1, 0, 0, 1, 0, 0, 0, -1, };
				eb_mul_frb(t[20], p, k20, sizeof(k20));
				signed char k21[] = {-1, 0, -1, 0, 1, 0, -1, 0, 0, 1, };
				eb_mul_frb(t[21], p, k21, sizeof(k21));
				signed char k22[] = {1, 0, -1, 0, 1, 0, -1, 0, 0, 1, };
				eb_mul_frb(t[22], p, k22, sizeof(k22));
				signed char k23[] = {-1, 0, 0, 0, -1, 0, 1, };
				eb_mul_frb(t[23], p, k23, sizeof(k23));
				signed char k24[] = {1, 0, 0, 0, -1, 0, 1, };
				eb_mul_frb(t[24], p, k24, sizeof(k24));
				signed char k25[] = {-1, 0, 1, 0, 1, 0, 0, -1, };
				eb_mul_frb(t[25], p, k25, sizeof(k25));
				signed char k26[] = {1, 0, 1, 0, 1, 0, 0, -1, };
				eb_mul_frb(t[26], p, k26, sizeof(k26));
				signed char k27[] =  {-1, 0, 0, -1, 0, -1, 0, -1, };
				eb_mul_frb(t[27], p, k27, sizeof(k27));
				signed char k28[] = {1, 0, 0, -1, 0, -1, 0, -1, };
				eb_mul_frb(t[28], p, k28, sizeof(k28));
				signed char k29[] = {-1, 0, -1, 0, 0, -1, 0, -1, };
				eb_mul_frb(t[29], p, k29, sizeof(k29));
				signed char k30[] = {1, 0, -1, 0, 0, -1, 0, -1, };
				eb_mul_frb(t[30], p, k30, sizeof(k30));
				signed char k31[] = {-1, 0, 0, 0, 0, 0, 1, };
				eb_mul_frb(t[31], p, k31, sizeof(k31));
				signed char k32[] = {1, 0, 0, 0, 0, 0, 1, };
				eb_mul_frb(t[32], p, k32, sizeof(k32));
				signed char k33[] = {-1, 0, 1, 0, 0, 1, };
				eb_mul_frb(t[33], p, k33, sizeof(k33));
				signed char k34[] = {1, 0, 1, 0, 0, 1, };
				eb_mul_frb(t[34], p, k34, sizeof(k34));
				signed char k35[] = {-1, 0, 0, 1, 0, 1, };
				eb_mul_frb(t[35], p, k35, sizeof(k35));
				signed char k36[] = {1, 0, 0, 1, 0, 1, };
				eb_mul_frb(t[36], p, k36, sizeof(k36));
				signed char k37[] = {-1, 0, -1, 0, -1, };
				eb_mul_frb(t[37], p, k37, sizeof(k37));
				signed char k38[] = {1, 0, -1, 0, -1, };
				eb_mul_frb(t[38], p, k38, sizeof(k38));
				signed char k39[] = {-1, 0, 0, 0, 1, 0, 1, };
				eb_mul_frb(t[39], p, k39, sizeof(k39));
				signed char k40[] = {1, 0, 0, 0, 1, 0, 1, };
				eb_mul_frb(t[40], p, k40, sizeof(k40));
				signed char k41[] = {-1, 0, 1, 0, -1, 0, -1, };
				eb_mul_frb(t[41], p, k41, sizeof(k41));
				signed char k42[] = {1, 0, 1, 0, -1, 0, -1, };
				eb_mul_frb(t[42], p, k42, sizeof(k42));
				signed char k43[] = {-1, 0, 0, -1, };
				eb_mul_frb(t[43], p, k43, sizeof(k43));
				signed char k44[] = {1, 0, 0, -1, };
				eb_mul_frb(t[44], p, k44, sizeof(k44));
				signed char k45[] = {-1, 0, -1, };
				eb_mul_frb(t[45], p, k45, sizeof(k45));
				signed char k46[] = {1, 0, -1, };
				eb_mul_frb(t[46], p, k46, sizeof(k46));
				signed char k47[] = {-1, 0, 0, 0, 0, -1, };
				eb_mul_frb(t[47], p, k47, sizeof(k47));
				signed char k48[] = {1, 0, 0, 0, 0, -1, };
				eb_mul_frb(t[48], p, k48, sizeof(k48));
				signed char k49[] = {-1, 0, 1, 0, 0, 0, -1, 0, -1, };
				eb_mul_frb(t[49], p, k49, sizeof(k49));
				signed char k50[] = {1, 0, 1, 0, 0, 0, -1, };
				eb_mul_frb(t[50], p, k50, sizeof(k50));
				signed char k51[] = {-1, 0, 0, 1, 0, 0, -1, };
				eb_mul_frb(t[51], p, k51, sizeof(k51));
				signed char k52[] = {1, 0, 0, 1, 0, 0, -1, };
				eb_mul_frb(t[52], p, k52, sizeof(k52));
				signed char k53[] = {-1, 0, -1, 0, 1, };
				eb_mul_frb(t[53], p, k53, sizeof(k53));
				signed char k54[] = {1, 0, -1, 0, 1, };
				eb_mul_frb(t[54], p, k54, sizeof(k54));
				signed char k55[] = {-1, 0, 0, 0, -1, 0, 0, 1, };
				eb_mul_frb(t[55], p, k55, sizeof(k55));
				signed char k56[] = {1, 0, 0, 0, -1, 0, 0, 1, };
				eb_mul_frb(t[56], p, k56, sizeof(k56));
				signed char k57[] = {-1, 0, 1, 0, 1, 0, -1, 0, -1, };
				eb_mul_frb(t[57], p, k57, sizeof(k57));
				signed char k58[] = {1, 0, 1, 0, 1, 0, -1, };
				eb_mul_frb(t[58], p, k58, sizeof(k58));
				signed char k59[] = {-1, 0, 0, -1, 0, 1, 0, 1, };
				eb_mul_frb(t[59], p, k59, sizeof(k59));
				signed char k60[] = {1, 0, 0, -1, 0, 1, 0, 1, };
				eb_mul_frb(t[60], p, k60, sizeof(k60));
				signed char k61[] = {-1, 0, -1, 0, 0, 1, 0, 1, };
				eb_mul_frb(t[61], p, k61, sizeof(k61));
				signed char k62[] = {1, 0, -1, 0, 0, 1, 0, 1, };
				eb_mul_frb(t[62], p, k62, sizeof(k62));
				signed char k63[] = {-1, 0, 0, 0, 0, 0, 0, 1, };
				eb_mul_frb(t[63], p, k63, sizeof(k63));*/
				eb_copy(t[0], p);
			}
			break;
#endif
		}
#if defined(EB_MIXED)
		if (w > 2) {
			eb_norm_sim(t + 1, t + 1, (1 << (w - 2)) - 1);
		}
#endif
	}
#endif /* EB_KBLTZ */
}

void eb_print(eb_t p) {
	fb_print(p->x);
	fb_print(p->y);
	fb_print(p->z);
}
