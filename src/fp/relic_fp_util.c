/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2013 RELIC Authors
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
 * Implementation of the prime field utilities.
 *
 * @version $Id$
 * @ingroup fp
 */

#include <inttypes.h>

#include "relic_core.h"
#include "relic_fp_low.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fp_copy(fp_t c, fp_t a) {
	int i;

	for (i = 0; i < FP_DIGS; i++, c++, a++) {
		*c = *a;
	}
}

void fp_zero(fp_t a) {
	for (int i = 0; i < FP_DIGS; i++, a++)
		*a = 0;
}

int fp_is_zero(fp_t a) {
	for (int i = 0; i < FP_DIGS; i++) {
		if (a[i] != 0) {
			return 0;
		}
	}
	return 1;
}

int fp_is_even(fp_t a) {
	if ((a[0] & 0x01) == 0) {
		return 1;
	}
	return 0;
}

int fp_test_bit(fp_t a, int bit) {
	int d;
	dig_t mask;

	SPLIT(bit, d, bit, FP_DIG_LOG);

	mask = ((dig_t)1) << bit;
	return (a[d] & mask) != 0;
}

int fp_get_bit(fp_t a, int bit) {
	int d;
	dig_t mask;

	SPLIT(bit, d, bit, FP_DIG_LOG);

	mask = (dig_t)1 << bit;

	return ((a[d] & mask) >> bit);
}

void fp_set_bit(fp_t a, int bit, int value) {
	int d;
	dig_t mask;

	SPLIT(bit, d, bit, FP_DIG_LOG);

	mask = (dig_t)1 << bit;

	if (value == 1) {
		a[d] |= mask;
	} else {
		a[d] &= ~mask;
	}
}

int fp_bits(fp_t a) {
	int i = FP_DIGS - 1;

	while (a[i] == 0) {
		i--;
	}

	return (i << FP_DIG_LOG) + util_bits_dig(a[i]);
}

void fp_set_dig(fp_t c, dig_t a) {
	fp_prime_conv_dig(c, a);
}

void fp_rand(fp_t a) {
	int bits, digits;

	rand_bytes((unsigned char *)a, FP_DIGS * sizeof(dig_t));

	SPLIT(bits, digits, FP_BITS, FP_DIG_LOG);
	if (bits > 0) {
		dig_t mask = ((dig_t)1 << (dig_t)bits) - 1;
		a[FP_DIGS - 1] &= mask;
	}

	while (fp_cmp(a, fp_prime_get()) != CMP_LT) {
		fp_subn_low(a, a, fp_prime_get());
	}
}

void fp_print(fp_t a) {
	int i;
	bn_t t;

	bn_null(t);

	TRY {
		bn_new(t);

#if FP_RDC == MONTY
		if (a != fp_prime_get()) {
			fp_prime_back(t, a);
		} else {
			t->used = FP_DIGS;
			fp_copy(t->dp, fp_prime_get());
		}
#else
		t->used = FP_DIGS;
		fp_copy(t->dp, a);
#endif

		for (i = FP_DIGS - 1; i >= 0; i--) {
#if WORD == 64
			if (i >= t->used) {
				util_print("%.*" PRIX64 " ", (int)(2 * (FP_DIGIT / 8)),
						(uint64_t)0);
			} else {
				util_print("%.*" PRIX64 " ", (int)(2 * (FP_DIGIT / 8)),
						(uint64_t)t->dp[i]);
			}
#else
			if (i >= t->used) {
				util_print("%.*" PRIX32 " ", (int)(2 * (FP_DIGIT / 8)),
						(uint32_t)0);
			} else {
				util_print("%.*" PRIX32 " ", (int)(2 * (FP_DIGIT / 8)),
						(uint32_t)t->dp[i]);
			}

#endif
		}
		util_print("\n");

	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(t);
	}
}

void fp_size(int *size, fp_t a, int radix) {
	bn_t t;

	bn_null(t);

	TRY {
		bn_new(t);

		fp_prime_back(t, a);

		bn_size_str(size, t, radix);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(t);
	}
}

void fp_read(fp_t a, const char *str, int len, int radix) {
	bn_t t;

	bn_null(t);

	TRY {
		bn_new(t);
		bn_read_str(t, str, len, radix);
		if (bn_is_zero(t)) {
			fp_zero(a);
		} else {
			if (t->used == 1) {
				fp_prime_conv_dig(a, t->dp[0]);
			} else {
				fp_prime_conv(a, t);
			}
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(t);
	}
}

void fp_write(char *str, int len, fp_t a, int radix) {
	bn_t t;

	bn_null(t);

	TRY {
		bn_new(t);

		fp_prime_back(t, a);

		bn_write_str(str, len, t, radix);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(t);
	}
}
