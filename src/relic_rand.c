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
 * Implementation of the pseudo-random number generator.
 *
 * @version $Id$
 * @ingroup rand
 */

#include <stdlib.h>
#include <stdint.h>

#include "relic_conf.h"
#include "relic_core.h"
#include "relic_rand.h"
#include "relic_md.h"
#include "relic_error.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#include <string.h>

/**
 * Size of the PRNG internal state in bytes.
 */
#define STATE_SIZE	    20

/**
 * Internal state of the PRNG.
 */
static unsigned char state[64];

#if SEED == DEV

#include <sys/stat.h>
#include <fcntl.h>
#include <unistd.h>

/**
 * The path to the char device that supplies entropy.
 */
#define RAND_PATH		"/dev/random"

#endif

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#include "relic_md.h"

void rand_init() {
	unsigned char buf[STATE_SIZE];

	memset(state, 0, sizeof(state));
#if SEED == DEV
	int rand_fd, c, l;
	rand_fd = open(RAND_PATH, O_RDONLY);
	if (rand_fd == -1) {
		THROW(ERR_NO_FILE);
	}
	l = 0;
	do {
		c = read(rand_fd, buf + l, STATE_SIZE - l);
		l += c;
		if (c == -1) {
			THROW(ERR_NO_READ);
		}
	} while (l < STATE_SIZE);
	if (rand_fd != -1) {
		close(rand_fd);
	}
#elif SEED == LIBC
#if OPSYS == FREEBSD
	srandom(1);
	for (int i = 0; i < STATE_SIZE; i++) {
		buf[i] = (unsigned char)random();
	}
#else
	srand(1);
	for (int i = 0; i < STATE_SIZE; i++) {
		buf[i] = (unsigned char)rand();
	}
#endif
#endif
	rand_seed(buf, STATE_SIZE);
}

void rand_clean() {
	memset(state, 0, sizeof(state));
}

void rand_seed(unsigned char *buf, int size) {
    int i;

    if (size < STATE_SIZE) {
    	THROW(ERR_INVALID);
    }

    /* XKEY = SEED  */
    for (i = 0; i < STATE_SIZE; i++) {
        state[i] = buf[i];
    }
    for (i = STATE_SIZE; i < 64; i++) {
    	state[i] = 0;
    }
}

void rand_bytes(unsigned char *buf, int size) {
    unsigned char carry, c0, c1, r0, r1;
    int i, j;
    unsigned char hash[20];

    j = 0;
    while (j < size) {
        /* x = G(t, XKEY) */
        md_map_shone_init();
        md_map_shone_update(state, 64);
        md_map_shone_state(hash);
        /* XKEY = (XKEY + x + 1) mod 2^b */
        carry = 1;
        for (i = 19; i >= 0; i--) {
    		r0 = (unsigned char)(state[i] + hash[i]);
    		c0 = (unsigned char)(r0 < hash[i] ? 1 : 0);
    		r1 = (unsigned char)(r0 + carry);
    		c1 = (unsigned char)(r1 < r0 ? 1 : 0);
    		carry = (unsigned char)(c0 | c1);
    		state[i] = r1;
        }
        for (i = 0; i < STATE_SIZE && j < size; i++) {
            buf[j] = hash[i];
            j++;
        }
    }
}
