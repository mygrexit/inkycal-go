package epd

func init() {
	traced["epd_5_in_83_V2"] = blackOnly(NewEPD5in83V2)
	// golden from tools/epdtrace/trace_5in83b_v2.py: upstream's own path crashes
	traced["epd5in83b_V2"] = blackOnly(NewEPD5in83bV2)
}
