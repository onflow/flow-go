/*
 * Copyright 2007 Project RELIC
 *
 * This file is part of RELIC. RELIC is legal property of its developers,
 * whose names are not listed here. Please refer to the COPYRIGHT file.
 *
 * RELIC is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * RELIC is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
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

#include <stdlib.h>
#include <stdio.h>
#include <string.h>

#include "relic_core.h"
#include "relic_fp.h"
#include "relic_fp_low.h"
#include "relic_rand.h"
#include "relic_util.h"
#include "relic_error.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fp_copy(fp_t c, fp_t a) {
	int i;

	for (i = 0; i < FP_DIGS; i++, c++, a++) {
		*c = *a;
	}
}

void fp_neg(fp_t c, fp_t a) {
	fp_subn_low(c, fp_prime_get(), a);
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
	int i, j;
	dig_t b;
	dig_t t;

	for (i = FP_DIGS - 1; i >= 0; i--) {
		t = a[i];
		if (t == 0) {
			continue;
		}
		b = (dig_t)1 << (FP_DIGIT - 1);
		j = FP_DIGIT - 1;
		while (!(t & b)) {
			j--;
			b >>= 1;
		}
		return (i << FP_DIG_LOG) + j + 1;
	}
	return 0;
}

void fp_set_dig(fp_t c, dig_t a) {
	fp_prime_conv_dig(c, a);
}

void fp_rand(fp_t a) {
	rand_bytes((unsigned char *)a, FP_DIGS * sizeof(dig_t));

	while (fp_cmp(a, fp_prime_get()) != CMP_LT) {
		fp_subn_low(a, a, fp_prime_get());
	}
}

void fp_print(fp_t a) {
	int i;
	bn_t t = NULL;

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
			if (i >= t->used) {
				util_print("%.*lX ", (int)(2 * sizeof(dig_t)),
						(unsigned long int)0);
			} else {
				util_print("%.*lX ", (int)(2 * sizeof(dig_t)),
						(unsigned long int)t->dp[i]);
			}
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
	bn_t t = NULL;

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
	bn_t t = NULL;

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
	bn_t t = NULL;

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
