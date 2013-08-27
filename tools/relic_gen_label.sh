#!/bin/bash

cat << PREAMBLE
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
 * Symbol renaming to a#undef clashes when simultaneous linking multiple builds.
 *
 * @version \$Id\$
 * @ingroup core
 */

#ifndef RELIC_LABEL_H
#define RELIC_LABEL_H

#include "relic_conf.h"

#define PREFIX(F)			_PREFIX(LABEL, F)
#define _PREFIX(A, B)		__PREFIX(A, B)
#define __PREFIX(A, B)		A ## _ ## B

/*============================================================================*/
/* Macro definitions                                                          */
/*============================================================================*/

#ifdef LABEL

PREAMBLE

REDEFIN() {
	cat "relic_$1.h.in" | grep "$1_" | grep -v define | grep -v typedef | grep -v '\\' | grep '(' | grep -v '^ \*' | sed 's/\*//' | sed -r 's/[a-z,_0-9]+ ([a-z,_,0-9]+)\(.*/\#undef \1/'
	echo
	cat "relic_$1.h.in" | grep "$1_" | grep -v define | grep -v typedef | grep -v '\\' | grep '(' | sed 's/\*//' | sed -r 's/[a-z,_,0-9]+ ([a-z,_,0-9]+)\(.*/\#define \1 \tPREFIX\(\1\)/'
	echo
}

REDEF() {
	cat "relic_$1.h" | grep "$1_" | grep -v define | grep -v typedef | grep -v '\\' | grep '(' | grep -v '^ \*' | sed 's/\*//' | sed -r 's/[a-z,_0-9]+ ([a-z,_,0-9]+)\(.*/\#undef \1/'
	echo
	cat "relic_$1.h" | grep "$1_" | grep -v define | grep -v typedef | grep -v '\\' | grep '(' | sed 's/\*//' | sed -r 's/[a-z,_,0-9]+ ([a-z,_,0-9]+)\(.*/\#define \1 \tPREFIX\(\1\)/'
	echo
}

REDEF_LOW() {
	cat "relic_$1.h" | grep "$1_" | grep -v define | grep -v typedef | grep -v '\\' | grep '(' | grep -v '^ \*' | sed 's/\*//' | sed -r 's/[a-z,_]+ ([a-z,_,0-9]+)\(.*/\#undef \1/'
	cat "low/relic_$1_low.h" | grep "$1_" | grep -v define | grep -v typedef | grep -v '\\' | grep -v '\}' | grep -v '^ \*' | sed 's/\*//' | sed -r 's/[a-z,_]+ ([a-z,_,0-9]+)\(.*/\#undef \1/'
	echo
	cat "relic_$1.h" | grep "$1_" | grep -v define | grep -v typedef | grep -v '\\' | grep '(' | grep -v '^ \*' | sed 's/\*//' | sed -r 's/[a-z,_]+ ([a-z,_,0-9]+)\(.*/\#define \1 \tPREFIX\(\1\)/'
	cat "low/relic_$1_low.h" | grep "$1_" | grep -v define | grep -v @version | grep -v typedef | grep -v '\\' | grep -v '\}' | grep -v '^ \*' | sed 's/\*//' | sed -r 's/[a-z,_]+ ([a-z,_,0-9]+)\(.*/\#define \1 \tPREFIX\(\1\)/'
	echo
}

REDEF core
REDEF arch
REDEF bench
REDEFIN conf
REDEF err
REDEF rand
REDEF pool
REDEF test
REDEF trace
REDEF util

echo "#undef dv_t"
echo "#define dv_t	PREFIX(dv_t)"
echo
REDEF_LOW dv

echo "#undef bn_st"
echo "#undef bn_t"
echo "#define bn_st	PREFIX(bn_st)"
echo "#define bn_t	PREFIX(bn_t)"
echo
REDEF_LOW bn

echo "#undef fp_st"
echo "#undef fp_t"
echo "#define fp_st	PREFIX(fp_st)"
echo "#define fp_t	PREFIX(fp_t)"
echo
REDEF_LOW fp

echo "#undef fp_st"
echo "#undef fp_t"
echo "#define fp_st	PREFIX(fp_st)"
echo "#define fp_t	PREFIX(fp_t)"
echo
REDEF_LOW fb

echo "#undef ep_st"
echo "#undef ep_t"
echo "#define ep_st	PREFIX(ep_st)"
echo "#define ep_t	PREFIX(ep_t)"
echo
REDEF eb

echo "#undef eb_st"
echo "#undef eb_t"
echo "#define eb_st	PREFIX(eb_st)"
echo "#define eb_t	PREFIX(eb_t)"
echo
REDEF eb

echo "#endif /* LABEL */"
echo
echo "#endif /* !RELIC_LABEL_H */"
