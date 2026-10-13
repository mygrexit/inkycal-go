package epd

func init() {
	traced["epd_4_in_2"] = blackOnly(NewEPD4in2)
	traced["epd_4_in_2_colour"] = NewEPD4in2Colour
}
