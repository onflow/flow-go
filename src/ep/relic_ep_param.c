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
 * Implementation of the prime elliptic curve utilities.
 *
 * @version $Id$
 * @ingroup ep
 */

#include <string.h>

#include "relic_core.h"
#include "relic_ep.h"
#include "relic_error.h"
#include "relic_conf.h"

#if defined(EP_STAND)

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#if defined(EP_ORDIN) && FP_PRIME == 192
/**
 * Parameters for the NIST P-192 binary elliptic curve.
 */
/** @{ */
#define NIST_P192_A		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFFFFFFFFFFFC"
#define NIST_P192_B		"64210519E59C80E70FA7E9AB72243049FEB8DEECC146B9B1"
#define NIST_P192_X		"188DA80EB03090F67CBF20EB43A18800F4FF0AFD82FF1012"
#define NIST_P192_Y		"07192B95FFC8DA78631011ED6B24CDD573F977A11E794811"
#define NIST_P192_R		"FFFFFFFFFFFFFFFFFFFFFFFF99DEF836146BC9B1B4D22831"
/** @} */
#endif

#if defined(EP_ORDIN) && FP_PRIME == 256
/**
 * Parameters for a pairing-friendly prime curve.
 */
/** @{ */
#define BNN_P256_A	"0"
#define BNN_P256_B	"16"
#define BNN_P256_X "1"
#define BNN_P256_Y "C7424FC261B627189A14E3433B4713E9C2413FCF89B8E2B178FB6322EFB2AB3"
#define BNN_P256_R "2523648240000001BA344D8000000007FF9F800000000010A10000000000000D"
/** @} */
#endif

#if ARCH == AVR

#include <avr/pgmspace.h>

/**
 * Copies a string from the text section to the destination vector.
 *
 * @param[out] dest		- the destination vector.
 * @param[in] src		- the pointer to the string stored on the text section.
 */
static void copy_from_rom(char *dest, const char *src) {
	char c;
	while ((c = pgm_read_byte(src++)))
		*dest++ = c;
	*dest = 0;
}

#endif

/**
 * Prepares a set of elliptic curve parameters.
 *
 * @param[out] STR		- the resulting prepared parameter.
 * @param[in] ID		- the parameter represented as a string.
 */
#if ARCH == AVR
#define PREPARE(STR, ID)													\
	copy_from_rom(STR, PSTR(ID));
#else
#define PREPARE(STR, ID)													\
	str = ID;
#endif

/**
 * Assigns a set of ordinary elliptic curve parameters.
 *
 * @param[in] CURVE		- the curve parameters to assign.
 * @param[in] FIELD		- the finite field identifier.
 */
#define ASSIGN(CURVE, FIELD)												\
	fp_param_set(FIELD);													\
	PREPARE(str, CURVE##_A);												\
	fp_read(a, str, strlen(str), 16);										\
	PREPARE(str, CURVE##_B);												\
	fp_read(b, str, strlen(str), 16);										\
	PREPARE(str, CURVE##_X);												\
	fp_read(g->x, str, strlen(str), 16);									\
	PREPARE(str, CURVE##_Y);												\
	fp_read(g->y, str, strlen(str), 16);									\
	PREPARE(str, CURVE##_R);												\
	bn_read_str(r, str, strlen(str), 16);									\

/**
 * Current configured elliptic curve parameters.
 */
static int param_id;

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void ep_param_set(int param) {
	int ordin = 0;
#if ARCH == AVR
	char str[2 * FB_DIGS + 1];
#else
	char *str;
#endif
	fp_t a, b;
	ep_t g;
	bn_t r;

	fp_null(a);
	fp_null(b);
	ep_null(g);
	bn_null(r);

	TRY {
		fp_new(a);
		fp_new(b);
		ep_new(g);
		bn_new(r);

		switch (param) {
#if defined(EP_ORDIN) && FP_PRIME == 192
			case NIST_P192:
				ASSIGN(NIST_P192, NIST_192);
				ordin = 1;
				break;
#endif
#if defined(EP_ORDIN) && FP_PRIME == 224
			case NIST_P224:
				ASSIGN(NIST_P224, NIST_224);
				ordin = 1;
				break;
#endif
#if defined(EP_ORDIN) && FB_PRIME == 256
			case NIST_P256:
				ASSIGN(NIST_P256, NIST_256);
				ordin = 1;
				break;
#endif
#if defined(EP_ORDIN) && FB_PRIME == 384
			case NIST_P384:
				ASSIGN(NIST_P384, NIST_384);
				ordin = 1;
				break;
#endif
#if defined(EP_ORDIN) && FP_PRIME == 521
			case NIST_P521:
				ASSIGN(NIST_P521, NIST_521);
				ordin = 1;
				break;
#endif
#if defined(EP_ORDIN) && FP_PRIME == 256
			case BNN_P256:
				ASSIGN(BNN_P256, BNN_256);
				pairf = 1;
				break;
			case BNP_P256:
				ASSIGN(BNP_P256, BNP_256);
				pairf = 1;
				break;
#endif
			default:
				(void)str;
				THROW(ERR_INVALID);
				break;
		}

		param_id = param;

		fp_zero(g->z);
		fp_set_dig(g->z, 1);
		g->norm = 1;

		ep_curve_set_ordin(a, b, g, r);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp_free(a);
		fp_free(b);
		ep_free(g);
		bn_free(r);
	}
}

#endif /* EP_STAND */

void ep_param_set_any() {
	int r0, r1;

	r0 = ep_param_set_any_ordin();
	r1 = ep_param_set_any_pairf();

	if (r0 == STS_ERR && r1 == STS_ERR) {
		THROW(ERR_NO_CURVE);
	}
}

int ep_param_set_any_ordin() {
	int r = STS_OK;
#if FP_PRIME == 192
	ep_param_set(NIST_P192);
#elif FP_PRIME == 224
	ep_param_set(NIST_P224);
#elif FP_PRIME == 256
	ep_param_set(NIST_P256);
#elif FP_PRIME == 384
	ep_param_set(NIST_P384);
#elif FP_PRIME == 521
	ep_param_set(NIST_P521);
#else
	r = STS_ERR;
#endif
	return r;
}

int ep_param_set_any_pairf() {
	int r = STS_OK;
#if FP_PRIME == 256
	ep_param_set(BNN_256)
#else
	r = STS_ERR;
#endif
	return r;
}

void ep_param_print() {
	switch (param_id) {
		case NIST_P192:
			util_print_banner("Curve NIST-P192:", 0);
			break;
		case NIST_P224:
			util_print_banner("Curve NIST-P224:", 0);
			break;
		case NIST_P256:
			util_print_banner("Curve NIST-P256:", 0);
			break;
		case NIST_P384:
			util_print_banner("Curve NIST-P384:", 0);
			break;
		case NIST_P521:
			util_print_banner("Curve NIST-P521:", 0);
			break;
		case BNN_P256:
			util_print_banner("Curve BNN-P256:", 0);
			break;
		case BNP_P256:
			util_print_banner("Curve BNP-P256:", 0);
			break;
	}
}
