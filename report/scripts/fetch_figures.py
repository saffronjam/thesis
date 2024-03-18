from pypdf import PdfReader, PdfWriter
import urllib.request
import os
import pdf2image

permalink = "https://docs.google.com/presentation/d/1RgcRZENtoHmxv5kiTlNwwcNEDgpMYzRaJOtbawGU5Vs/export?format=pdf"
first_slide = 2
last_slide = -1
pdf_local_name = "figures.pdf"
out_dir = "../img"



def fetch_pdf():
    urllib.request.urlretrieve(permalink, os.path.join("out" ,pdf_local_name))


def main():
    # Create dir out if not exists
    if not os.path.exists("out"):
        os.makedirs("out")

    print("Fetching figures...")
    fetch_pdf()

    print("Extracting figures...")
    reader = PdfReader(os.path.join("out" ,pdf_local_name))

    # The pdf should be structured as text>figure>text>figure>text>figure

    global last_slide
    if last_slide == -1:
        last_slide = len(reader.pages)
    
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
            
            writer.write("out/figure" + str(i) + ".pdf")

            images = pdf2image.convert_from_path("out/figure" + str(i) + ".pdf")
            images[0].save("out/figure" + str(i) + ".png", "PNG")

            # Use imagemagick to crop unnecessary content around the image
            output_file = os.path.join(out_dir, filename + ".png")
            os.system("convert out/figure" + str(i) + ".png -trim " + output_file)

    # Remove out folder
    os.system("rm -rf out")

    print("Done")



if __name__ == "__main__":
    main()