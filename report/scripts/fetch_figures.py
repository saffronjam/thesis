from pypdf import PdfReader, PdfWriter
import urllib.request
import os
import pdf2image

permalink = "https://docs.google.com/presentation/d/1RgcRZENtoHmxv5kiTlNwwcNEDgpMYzRaJOtbawGU5Vs/export?format=pdf"
first_slide = 2
last_slide = 8
pdf_local_name = "figures.pdf"
out_dir = "../img"



def fetch_pdf():
    urllib.request.urlretrieve(permalink, pdf_local_name)


def main():
    print("Fetching figures...")
    fetch_pdf()

    print("Extracting figures...")
    reader = PdfReader(pdf_local_name)

    # The pdf should be structured as text>figure>text>figure>text>figure
    
    for i in range(last_slide - first_slide):
        if i % 2 == 0:
            # Text, extract text
            filename = reader.pages[first_slide + i].extract_text()
            # Trim spaces around filename
            filename = filename.strip()
            print(filename)
        else:
            # Figure, split pdf and write to file
            # Then use pdf2image to convert to image
            # Then use imagemagick to convert to png
            writer = PdfWriter()
            writer.add_page(reader.pages[first_slide + i])

            # Create dir out if not exists
            if not os.path.exists("out"):
                os.makedirs("out")
            
            writer.write("out/figure" + str(i) + ".pdf")
            print("Wrote figure" + str(i) + ".pdf")

            images = pdf2image.convert_from_path("out/figure" + str(i) + ".pdf")
            images[0].save("out/figure" + str(i) + ".png", "PNG")

            # Use imagemagick to crop unnecessary content around the image
            output_file = os.path.join(out_dir, filename + ".png")
            os.system("convert out/figure" + str(i) + ".png -trim " + output_file)

            # Remove temporary pdf and png
            os.remove("out/figure" + str(i) + ".pdf")
            os.remove("out/figure" + str(i) + ".png")

    print("Done")



if __name__ == "__main__":
    main()