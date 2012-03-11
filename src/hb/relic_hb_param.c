/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2011 RELIC Authors
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
 * Implementation of the binary hyperelliptic curve utilities.
 *
 * @version $Id$
 * @ingroup eb
 */

#include <string.h>

#include "relic_core.h"
#include "relic_hb.h"
#include "relic_util.h"
#include "relic_error.h"
#include "relic_conf.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#if defined(HB_SUPER) && FB_POLYN == 367
/**
 * Parameters for the MIRACL supersingular pairing-friendly elliptic curve over
 * GF(2^1223).
 */
/** @{ */
#define ETAT_S367_F3	"1"
#define ETAT_S367_F1	"0"
#define ETAT_S367_F0	"0"
#define ETAT_S367_U1	"63ED2493096154F4013F5A7FDE4A19C29BD82583D9D73C8D449EFC472D32511F80C642932686541D085F089DCC89"
#define ETAT_S367_U0	"7B256D9AAF3A574D6912D4F72CCB1D2E60736EB0A2316A8D039B8C8F38BD7123B5EF6DEDF68E2568A6E5C373DB2D"
#define ETAT_S367_V1	"6AAFF034625346A49A65DF5C70F98C962D394AD76FA9DEFA4A98E7DE64A7192D99B7EEDD4A7DC2530E1F9B9B2F88"
#define ETAT_S367_V0	"699BAB71F3DF73970CC5189283AEB6CD5CEB32CC712B668EED99D6E50AB2A97E24BE21BC929E42346A31A47EC6FC"
#define ETAT_S367_R		"2F2EBD8198A8E59E2DE4FCFFF8B1ED8BCDD07A37AA15581182E34B202BDBDA58A5A84BCA3DEBA2E069FCA24BB6D34C08D509766365751F44F3A917EF88854095E028E1BFDED2F2569A945336F89D3E641CC6FC789FD9055"
#define ETAT_S367_H		"15B3F2ECFD"
/** @} */
#endif

#if defined(HB_SUPER) && FB_POLYN == 439
/**
 * Parameters for the MIRACL supersingular pairing-friendly elliptic curve over
 * GF(2^1223).
 */
/** @{ */
#define ETAT_S439_F3	"1"
#define ETAT_S439_F1	"0"
#define ETAT_S439_F0	"0"
#define ETAT_S439_U1	"1B2D4798B209328AA3C482C2478176B439FC6FD248896534721CADB3976E14F72474A03347FCF9103C0B841CFB2DAF735BF386F44408B5"
#define ETAT_S439_U0	"1AB415010174AFEC4EB4A273C0C2B8E1F2850FDCDAB0FCFBB4D457341F8CC0BCD673A2BE0FB5203D533F5A0F048F58951B2C00FE9981DD"
#define ETAT_S439_V1	"EFF6A17976A7337D4BD67E9CE9AC730D42AD521D7F14BB03E19A071E578828AEC585A5880AF55C30437D557271F2E1D59AB27FF3825B2"
#define ETAT_S439_V0	"5BEB7C742CD1F7A773EE921BF2B0A2C108C9550976A2A0674F65E5290539C3E2B824F1BEC9B11448040BC574DE512E6AC067CB4B6029"
#define ETAT_S439_R		"4EC4EC4EC4EC4EC4EC4EC4EC4EC4EC4EC4EC4EC4EC4EC4EC4EC4EC589D89D89D89D89D89D89D89D89D89D89D89D89D89D89D89D89D89D93B13B13B13B13B13B13B13B13B13B13B13B13B13B13B13B13B13B14EC4EC4EC4EC4EC4EC4EC4EC4EC4EC4EC4EC4EC4EC4EC4EC4EC4EC5"
#define ETAT_S439_H		"13"
/** @} */
#endif

/**
 * Assigns a set of supersingular hyperelliptic curve parameters.
 *
 * @param[in] CURVE		- the curve parameters to assign.
 * @param[in] FIELD		- the finite field identifier.
 */
#define ASSIGNS(CURVE, FIELD)												\
	fb_param_set(FIELD);													\
	PREPARE(str, CURVE##_F0);												\
	fb_read(f0, str, strlen(str), 16);										\
	PREPARE(str, CURVE##_F1);												\
	fb_read(f1, str, strlen(str), 16);										\
	PREPARE(str, CURVE##_F3);												\
	fb_read(f3, str, strlen(str), 16);										\
	PREPARE(str, CURVE##_U0);												\
	fb_read(g->u0, str, strlen(str), 16);									\
	PREPARE(str, CURVE##_U1);												\
	fb_read(g->u1, str, strlen(str), 16);									\
	PREPARE(str, CURVE##_V0);												\
	fb_read(g->v0, str, strlen(str), 16);									\
	PREPARE(str, CURVE##_V1);												\
	fb_read(g->v1, str, strlen(str), 16);									\
	PREPARE(str, CURVE##_R);												\
	bn_read_str(r, str, strlen(str), 16);									\
	PREPARE(str, CURVE##_H);												\
	bn_read_str(h, str, strlen(str), 16);									\

/**
 * Current configured hyperelliptic curve parameters.
 */
static int param_id;

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

int hb_param_get() {
	return param_id;
}

void hb_param_set(int param) {
	int super = 0;
#if ARCH == AVR
	char str[2 * FB_DIGS + 1];
#else
	char *str;
#endif
	fb_t f0, f1, f3;
	hb_t g;
	bn_t r;
	bn_t h;

	fb_null(f0);
	fb_null(f1);
	fb_null(f3);
	hb_null(g);
	bn_null(r);
	bn_null(h);

	TRY {
		fb_new(f0);
		fb_new(f1);
		fb_new(f3);
		hb_new(g);
		bn_new(r);
		bn_new(h);

		switch (param) {
#if defined(HB_SUPER) && FB_POLYN == 367
			case ETAT_S367:
				ASSIGNS(ETAT_S367, TRINO_367);
				super = 1;
				break;
#endif
#if defined(HB_SUPER) && FB_POLYN == 439
			case ETAT_S439:
				ASSIGNS(ETAT_S439, TRINO_439);
				super = 1;
				break;
#endif
			default:
				(void)str;
				THROW(ERR_NO_VALID);
				break;
		}

		param_id = param;
		g->deg = 0;

#if defined(HB_SUPER)
		if (super) {
			hb_curve_set_super(f3, f1, f0, g, r, h);
		}
#endif
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb_free(f0);
		fb_free(f1);
		fb_free(f3);
		hb_free(g);
		bn_free(r);
		bn_free(h);
	}
}

int hb_param_set_any() {
	int r0;

	r0 = hb_param_set_any_super();
	if (r0 == STS_ERR) {
		return STS_ERR;
	}
	return STS_OK;
}

int hb_param_set_any_super() {
	int r = STS_OK;
#if FB_POLYN == 367
	hb_param_set(ETAT_S367);
#elif FB_POLYN == 439
	hb_param_set(ETAT_S439);
#else
	r = STS_ERR;
#endif
	return r;
}

void hb_param_print() {
	switch (param_id) {
		case ETAT_S367:
			util_banner("Curve ETAT-S367:", 0);
			break;
		case ETAT_S439:
			util_banner("Curve ETAT-S439:", 0);
			break;
	}
}

int hb_param_level() {
	switch (param_id) {
		case ETAT_S367:
			return 128;
		case ETAT_S439:
			return 136;
	}
	return 0;
}
